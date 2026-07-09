//go:build integration

// handler_org_customfields_integration_test.go — round-trip + sort/filter
// vocabulary coverage for organization custom-field values (CF-T05 Task 3).
// Proves the cf_<slug> wire value round-trips through create/get/list/update,
// that an active custom column is a legal sort AND filter field while a retired
// one is refused (422 sort_field_not_allowed / filter_field_not_allowed), and
// that the organization-360 composite read still carries its
// relationships/deals/activities arrays (organizationDetailResponse.MarshalJSON).
package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedOrgCFWorkspaceAndUser creates a fresh workspace + app_user (both real
// uuids) so customfields.Create — whose catalog row's created_by is a FK to
// app_user(id) — can run its real ALTER TABLE + catalog insert.
func seedOrgCFWorkspaceAndUser(t *testing.T, db *sql.DB) (wsID, userID string) {
	t.Helper()
	wsID, userID = ids.New(), ids.New()
	if _, err := db.Exec(`INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		wsID, "org-cf-ws-"+wsID, "org-cf-ws-"+wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		userID, wsID, "u"+userID+"@t.test", "U"); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}
	return wsID, userID
}

func TestOrganizationHandler_CustomFields_RoundTripSortFilterAndRetire(t *testing.T) {
	db := openDealTestDB(t)
	wsID, userID := seedOrgCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: userID})

	// A real active text column on organization (runs the ALTER TABLE).
	field, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object:     "organization",
		Label:      "Tier " + time.Now().Format("150405.000000000"),
		Type:       customfields.TypeText,
		Source:     "ui",
		CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("seed custom field: %v", err)
	}
	col := field.ColumnName // e.g. "cf_tier_150405_000000000"

	h := orgHandlerForTest(db, orgAdapters.NewOrgStore(db))

	doJSON := func(method, target string, body map[string]any, headers map[string]string) *httptest.ResponseRecorder {
		var rdr *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		} else {
			rdr = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(method, target, rdr)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	// --- Create org A with cf=alpha (201 echoes it). ---
	wA := doJSON(http.MethodPost, "/organizations", map[string]any{
		"display_name": "Alpha Co", "source": "test", "captured_by": "human:test", col: "alpha",
	}, nil)
	if wA.Code != http.StatusCreated {
		t.Fatalf("POST /organizations (A): got %d: %s", wA.Code, wA.Body.String())
	}
	var createdA map[string]any
	if err := json.NewDecoder(wA.Body).Decode(&createdA); err != nil {
		t.Fatal(err)
	}
	if createdA[col] != "alpha" {
		t.Fatalf("created A %s = %#v, want \"alpha\"", col, createdA[col])
	}
	idA, _ := createdA["id"].(string)

	// --- Create org B with cf=beta (for the filter narrowing case). ---
	wB := doJSON(http.MethodPost, "/organizations", map[string]any{
		"display_name": "Beta Co", "source": "test", "captured_by": "human:test", col: "beta",
	}, nil)
	if wB.Code != http.StatusCreated {
		t.Fatalf("POST /organizations (B): got %d: %s", wB.Code, wB.Body.String())
	}
	var createdB map[string]any
	_ = json.NewDecoder(wB.Body).Decode(&createdB)
	idB, _ := createdB["id"].(string)

	// --- Get A: includes cf value + composite arrays still present. ---
	wGet := doJSON(http.MethodGet, "/organizations/"+idA, nil, nil)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GET /organizations/{A}: got %d: %s", wGet.Code, wGet.Body.String())
	}
	var gotA map[string]any
	if err := json.NewDecoder(wGet.Body).Decode(&gotA); err != nil {
		t.Fatal(err)
	}
	if gotA[col] != "alpha" {
		t.Fatalf("get A %s = %#v, want \"alpha\"", col, gotA[col])
	}
	for _, key := range []string{"relationships", "deals", "activities"} {
		if _, ok := gotA[key]; !ok {
			t.Fatalf("composite key %q missing from GET /organizations/{id}: %s", key, wGet.Body.String())
		}
	}

	// --- List includes the cf value. ---
	wList := doJSON(http.MethodGet, "/organizations", nil, nil)
	if wList.Code != http.StatusOK {
		t.Fatalf("GET /organizations: got %d: %s", wList.Code, wList.Body.String())
	}
	if got := customFieldForID(t, wList.Body.Bytes(), idA, col); got != "alpha" {
		t.Fatalf("list A %s = %#v, want \"alpha\"", col, got)
	}

	// --- Sort by the active column: 200. ---
	wSort := doJSON(http.MethodGet, "/organizations?sort="+col, nil, nil)
	if wSort.Code != http.StatusOK {
		t.Fatalf("GET /organizations?sort=%s: got %d: %s", col, wSort.Code, wSort.Body.String())
	}

	// --- Update A's cf value: 200, new value sticks. ---
	wUp := doJSON(http.MethodPatch, "/organizations/"+idA, map[string]any{col: "gamma"}, nil)
	if wUp.Code != http.StatusOK {
		t.Fatalf("PATCH /organizations/{A}: got %d: %s", wUp.Code, wUp.Body.String())
	}
	wGet2 := doJSON(http.MethodGet, "/organizations/"+idA, nil, nil)
	var gotA2 map[string]any
	_ = json.NewDecoder(wGet2.Body).Decode(&gotA2)
	if gotA2[col] != "gamma" {
		t.Fatalf("after update A %s = %#v, want \"gamma\"", col, gotA2[col])
	}

	// --- Filter by the active column: narrows to the matching row (B=beta). ---
	wFilter := doJSON(http.MethodGet, "/organizations?"+col+"=beta", nil, nil)
	if wFilter.Code != http.StatusOK {
		t.Fatalf("GET /organizations?%s=beta: got %d: %s", col, wFilter.Code, wFilter.Body.String())
	}
	ids := listedIDs(t, wFilter.Body.Bytes())
	if len(ids) != 1 || ids[0] != idB {
		t.Fatalf("filter %s=beta returned %v, want exactly [%s]", col, ids, idB)
	}

	// --- Retire the field. ---
	if _, err := customfields.Retire(ctx, db, field.ID); err != nil {
		t.Fatalf("retire field: %v", err)
	}

	// Get no longer shows the key.
	wGet3 := doJSON(http.MethodGet, "/organizations/"+idA, nil, nil)
	if wGet3.Code != http.StatusOK {
		t.Fatalf("GET after retire: got %d: %s", wGet3.Code, wGet3.Body.String())
	}
	var gotA3 map[string]any
	_ = json.NewDecoder(wGet3.Body).Decode(&gotA3)
	if _, ok := gotA3[col]; ok {
		t.Fatalf("retired column %s still present on GET: %s", col, wGet3.Body.String())
	}

	// Sort by the retired column is refused.
	wSortR := doJSON(http.MethodGet, "/organizations?sort="+col, nil, nil)
	if wSortR.Code != http.StatusUnprocessableEntity {
		t.Fatalf("GET ?sort=%s after retire: got %d, want 422: %s", col, wSortR.Code, wSortR.Body.String())
	}
	assertProblemCode(t, wSortR.Body.Bytes(), "sort_field_not_allowed")

	// Filter by the retired column is refused.
	wFilterR := doJSON(http.MethodGet, "/organizations?"+col+"=beta", nil, nil)
	if wFilterR.Code != http.StatusUnprocessableEntity {
		t.Fatalf("GET ?%s=beta after retire: got %d, want 422: %s", col, wFilterR.Code, wFilterR.Body.String())
	}
	assertProblemCode(t, wFilterR.Body.Bytes(), "filter_field_not_allowed")
}

// customFieldForID returns the cf column value of the listed row with the given
// id, or nil if that row is absent.
func customFieldForID(t *testing.T, body []byte, id, col string) any {
	t.Helper()
	for _, row := range decodeListData(t, body) {
		if row["id"] == id {
			return row[col]
		}
	}
	return nil
}

func listedIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var out []string
	for _, row := range decodeListData(t, body) {
		if s, ok := row["id"].(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func decodeListData(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	return resp.Data
}

func assertProblemCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var p struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	if p.Code != want {
		t.Fatalf("problem code = %q, want %q (body=%s)", p.Code, want, string(body))
	}
}
