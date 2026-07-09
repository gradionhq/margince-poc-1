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
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func seedPersonCustomField(t *testing.T, db *sql.DB, wsID, userID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO custom_field (workspace_id, object, slug, label, type, column_name, created_by)
		VALUES ($1::uuid,'person','score','Score','number','cf_score',$2::uuid)`,
		wsID, userID)
	if err != nil {
		t.Fatalf("seed custom field: %v", err)
	}
}

func TestPersonHandler_CustomFields_RoundTripAndSortVocabulary(t *testing.T) {
	db := openTestDB(t)
	wsID := "00000000-0000-0000-0000-000000000051"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)
	seedPersonCustomField(t, db, wsID, "human:test")

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
	if err := json.NewDecoder(getW.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.CustomFields["cf_score"] != float64(42) {
		t.Fatalf("get cf_score = %#v, want 42", got.CustomFields["cf_score"])
	}
	if len(got.Relationships) != 0 || len(got.Deals) != 0 || len(got.Activities) != 0 {
		t.Fatalf("composite arrays should remain present and empty, got %+v", got)
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
	seedPersonCustomField(t, db, wsID, "human:test")

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	personStore := people.NewPersonStore(db)
	relStore := relationships.NewRelationshipStore(db)
	dealStore := deals.NewDealStore(db)
	activityStore := activities.NewActivityStore(db)
	h := NewPersonHandler(personStore, relStore, dealStore, activityStore, &crmapprovals.DBVerifier{DB: db})

	p, err := personStore.Create(ctx, people.Person{WorkspaceID: wsID, FullName: "Composite", Source: "test", CapturedBy: "human:test", CustomFields: map[string]any{"cf_score": 1}}, nil)
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
