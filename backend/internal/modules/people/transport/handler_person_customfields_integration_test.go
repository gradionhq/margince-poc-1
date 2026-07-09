//go:build integration

package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/lib/pq"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedPersonCustomField seeds an active custom field on person via
// customfields.Create — the one chokepoint allowed to run the ALTER TABLE —
// per the plan's own instruction, rather than a hand-written catalog INSERT
// that can drift from that engine's real column set (created_by is a uuid
// FK to app_user, not a free-text principal id like person.captured_by).
// label must be unique per call within a test binary run: the underlying
// engine ALTER TABLEs a real, workspace-independent column onto the shared
// person table (CF-T03), so two calls with the same label collide even
// across different workspace IDs.
func seedPersonCustomField(t *testing.T, db *sql.DB, wsID, label string) customfields.Created {
	t.Helper()
	userID := ids.New()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		userID, wsID, "u"+userID+"@t.test", "U"); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: userID})
	created, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object:     "person",
		Label:      label,
		Type:       customfields.TypeNumber,
		Source:     "test",
		CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("seed custom field: %v", err)
	}
	return created
}

func TestPersonHandler_CustomFields_RoundTripAndSortVocabulary(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-000000000051"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	seedPersonCustomField(t, db, wsID, "Score")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	createBody := map[string]any{
		"full_name":   "Custom Field Person",
		"source":      "test",
		"captured_by": "human:test",
		"cf_score":    42,
	}
	b, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewReader(b))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /people: got %d: %s", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created["cf_score"] != float64(42) {
		t.Fatalf("created cf_score = %#v, want 42", created["cf_score"])
	}
	id, _ := created["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/people/"+id, nil)
	getReq = getReq.WithContext(ctx)
	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: got %d: %s", getW.Code, getW.Body.String())
	}
	var got people.Person
	getBody := getW.Body.Bytes()
	if err := json.Unmarshal(getBody, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Relationships) != 0 || len(got.Deals) != 0 || len(got.Activities) != 0 {
		t.Fatalf("composite arrays should remain present and empty, got %+v", got)
	}
	// Person.UnmarshalJSON deliberately never populates CustomFields (it is
	// json:"-" and production code never decodes wire JSON into a Person) —
	// decode into a plain map to assert the wire-level cf_score value instead.
	var gotRaw map[string]any
	if err := json.Unmarshal(getBody, &gotRaw); err != nil {
		t.Fatal(err)
	}
	if gotRaw["cf_score"] != float64(42) {
		t.Fatalf("get cf_score = %#v, want 42", gotRaw["cf_score"])
	}

	updateBody := map[string]any{"cf_score": 7}
	ub, _ := json.Marshal(updateBody)
	upReq := httptest.NewRequest(http.MethodPatch, "/people/"+id, bytes.NewReader(ub))
	upReq.Header.Set("If-Match", "1")
	upReq = upReq.WithContext(ctx)
	upW := httptest.NewRecorder()
	h.ServeHTTP(upW, upReq)
	if upW.Code != http.StatusOK {
		t.Fatalf("PATCH /people/{id}: got %d: %s", upW.Code, upW.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/people?sort=cf_score", nil)
	listReq = listReq.WithContext(ctx)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("GET /people?sort=cf_score: got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp map[string]any
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	data, ok := listResp["data"].([]any)
	if !ok || len(data) == 0 {
		t.Fatalf("expected list data, got %#v", listResp["data"])
	}
}

func TestPersonHandler_Get_CompositeKeepsArrays(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-000000000052"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	seedPersonCustomField(t, db, wsID, "Rank")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	personStore := people.NewPersonStore(db)
	relStore := relationships.NewRelationshipStore(db)
	dealStore := deals.NewDealStore(db)
	activityStore := activities.NewActivityStore(db)
	h := NewPersonHandler(personStore, relStore, dealStore, activityStore, &crmapprovals.DBVerifier{DB: db})

	p, err := personStore.Create(ctx, people.Person{WorkspaceID: wsID, FullName: "Composite", Source: "test", CapturedBy: "human:test", CustomFields: map[string]any{"cf_rank": 1}}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/people/"+p.ID, nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"relationships", "deals", "activities"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing %q in composite response: %s", key, w.Body.String())
		}
	}
}

func TestPersonHandler_CustomFields_RetiredFieldHiddenAndSortRefused(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-000000000053"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	field := seedPersonCustomField(t, db, wsID, "Tier")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	createBody := map[string]any{
		"full_name":   "Retired Field Person",
		"source":      "test",
		"captured_by": "human:test",
		"cf_tier":     9,
	}
	b, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewReader(b))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /people: got %d: %s", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created["cf_tier"] != float64(9) {
		t.Fatalf("created cf_tier = %#v, want 9", created["cf_tier"])
	}
	id, _ := created["id"].(string)

	if _, err := customfields.Retire(ctx, db, field.ID); err != nil {
		t.Fatalf("retire custom field: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/people/"+id, nil)
	getReq = getReq.WithContext(ctx)
	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: got %d: %s", getW.Code, getW.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(getW.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["cf_tier"]; ok {
		t.Fatalf("retired field cf_tier still present in response: %s", getW.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/people?sort=cf_tier", nil)
	listReq = listReq.WithContext(ctx)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusUnprocessableEntity {
		t.Fatalf("GET /people?sort=cf_tier: got %d, want 422: %s", listW.Code, listW.Body.String())
	}
	var problem map[string]any
	if err := json.NewDecoder(listW.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem["code"] != "sort_field_not_allowed" {
		t.Fatalf("problem code = %#v, want sort_field_not_allowed: %s", problem["code"], listW.Body.String())
	}
}

// TestPersonHandler_CustomFields_ListByCustomColumn_PaginatesWithoutSkippingOrDuplicating
// is the regression test for the FINAL GATE finding: listByCustomColumn used
// to page with an id-keyset cursor (`id::text > $2`) over a query ORDER BY
// the custom column — since the seek key didn't match the sort key, a row
// whose id sorted below the previous page's last id (but whose cf_ value
// sorted later) could be skipped entirely. Three people are seeded with
// cf_ values that deliberately sort in the OPPOSITE order of their id/
// creation order, and limit=1 forces every row onto its own page, so an
// id-keyset bug would visibly skip or duplicate a row.
func TestPersonHandler_CustomFields_ListByCustomColumn_PaginatesWithoutSkippingOrDuplicating(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-000000000054"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	seedPersonCustomField(t, db, wsID, "Rank2")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	// Created in this order (so ids ascend in this order), but cf_rank2
	// values ascend in the REVERSE order — an id-keyset seek would walk id
	// order, not cf_rank2 order, silently skipping rows.
	seeded := []struct {
		name  string
		value int
	}{
		{"Page A", 30},
		{"Page B", 20},
		{"Page C", 10},
	}
	for _, s := range seeded {
		createBody := map[string]any{
			"full_name":   s.name,
			"source":      "test",
			"captured_by": "human:test",
			"cf_rank2":    s.value,
		}
		b, _ := json.Marshal(createBody)
		req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewReader(b))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("POST /people (%s): got %d: %s", s.name, w.Code, w.Body.String())
		}
	}

	seen := map[string]bool{}
	var order []string
	cursor := ""
	for page := 0; page < 10; page++ {
		url := "/people?sort=cf_rank2&limit=1"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: got %d: %s", url, w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		data, _ := resp["data"].([]any)
		for _, item := range data {
			m, _ := item.(map[string]any)
			fullName, _ := m["full_name"].(string)
			if seen[fullName] {
				t.Fatalf("duplicate row across pages: %q (order so far: %v)", fullName, order)
			}
			seen[fullName] = true
			order = append(order, fullName)
		}
		pageMeta, _ := resp["page"].(map[string]any)
		nextCursor, _ := pageMeta["next_cursor"].(string)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	if len(order) != len(seeded) {
		t.Fatalf("expected all %d seeded rows across pages (no skip), got %d: %v", len(seeded), len(order), order)
	}
	// cf_rank2 ascends 10, 20, 30 -> Page C, Page B, Page A.
	want := []string{"Page C", "Page B", "Page A"}
	for i, name := range want {
		if order[i] != name {
			t.Fatalf("page order[%d] = %q, want %q (full order %v)", i, order[i], name, order)
		}
	}
}

// TestPersonHandler_List_SortStrength_IncludesCustomFields is the regression
// test for the FINAL GATE finding that listByStrength never attached
// CustomFields at all: GET /people?sort=strength (and -strength) used to
// return persons with CustomFields == nil (and no cf_<slug> key on the
// wire), even though the default list and the custom-column sort both
// already attached them correctly.
func TestPersonHandler_List_SortStrength_IncludesCustomFields(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-000000000055"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	seedPersonCustomField(t, db, wsID, "Strength")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	createBody := map[string]any{
		"full_name":   "Strength Sorted Person",
		"source":      "test",
		"captured_by": "human:test",
		"cf_strength": 99,
	}
	b, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewReader(b))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /people: got %d: %s", w.Code, w.Body.String())
	}

	for _, sortVal := range []string{"strength", "-strength"} {
		listReq := httptest.NewRequest(http.MethodGet, "/people?sort="+sortVal, nil)
		listReq = listReq.WithContext(ctx)
		listW := httptest.NewRecorder()
		h.ServeHTTP(listW, listReq)
		if listW.Code != http.StatusOK {
			t.Fatalf("GET /people?sort=%s: got %d: %s", sortVal, listW.Code, listW.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(listW.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		data, ok := resp["data"].([]any)
		if !ok || len(data) == 0 {
			t.Fatalf("sort=%s: expected list data, got %#v", sortVal, resp["data"])
		}
		found := false
		for _, item := range data {
			m, _ := item.(map[string]any)
			if m["full_name"] == "Strength Sorted Person" {
				found = true
				if m["cf_strength"] != float64(99) {
					t.Fatalf("sort=%s: cf_strength = %#v, want 99: %s", sortVal, m["cf_strength"], listW.Body.String())
				}
			}
		}
		if !found {
			t.Fatalf("sort=%s: seeded person not found in response: %s", sortVal, listW.Body.String())
		}
	}
}
