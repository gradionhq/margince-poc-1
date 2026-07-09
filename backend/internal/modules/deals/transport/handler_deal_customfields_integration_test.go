//go:build integration

package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// cfHandlerTestUserID is a valid UUID-format user id seeded as an app_user
// row so customfields.Create's created_by (cast ::uuid) is satisfiable —
// mirrors handler_deal_advance_integration_test.go's advHandlerTestAgentID.
const cfHandlerTestUserID = "00000000-0000-0000-0000-0000000cf001"

// TestDealHandler_CustomFields_RoundTripAndVocabulary proves CF-T05 Task 4's
// deal wiring end to end: create/get/list/sort/update/filter all surface an
// active cf_* column on the wire, and retiring the field both hides its
// value and refuses it from the sort/filter vocabulary the same way an
// unknown field always was.
func TestDealHandler_CustomFields_RoundTripAndVocabulary(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "cf")
	store := adapters.NewDealStore(db)
	h := dealHandlerForTest(db, store)

	if _, err := db.Exec(`INSERT INTO app_user (id, workspace_id, email, display_name)
		VALUES ($1, $2, 'cf-handler-it@test.example', 'CF Handler IT')
		ON CONFLICT (id) DO NOTHING`, cfHandlerTestUserID, dealTestWorkspaceID); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}

	cfCtx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: cfHandlerTestUserID})
	tag := time.Now().Format("150405.000000000")
	field, err := customfields.Create(cfCtx, db, customfields.FieldSpec{
		Object: "deal", Label: "Deal Score " + tag, Type: customfields.TypeText,
		Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("seed custom field: %v", err)
	}
	cfKey := field.ColumnName

	createBody, _ := json.Marshal(map[string]any{
		"name": "CF deal", "pipeline_id": pipelineID, "stage_id": stageID,
		"amount_minor": 1000, "currency": "EUR",
		"source": "test", "captured_by": "human:test",
		cfKey: "alpha",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(createBody))
	createReq = withDealWorkspace(createReq)
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", createW.Code, createW.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created[cfKey] != "alpha" {
		t.Fatalf("expected %s=alpha in create response, got %v", cfKey, created[cfKey])
	}
	dealID := created["id"].(string)

	getReq := httptest.NewRequest(http.MethodGet, "/deals/"+dealID, nil)
	getReq = withDealWorkspace(getReq)
	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", getW.Code, getW.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if got[cfKey] != "alpha" {
		t.Fatalf("expected %s=alpha on get, got %v", cfKey, got[cfKey])
	}
	if _, ok := got["stakeholders"].([]any); !ok {
		t.Fatalf("expected stakeholders array present (dealDetailResponse.MarshalJSON fix), got %v", got["stakeholders"])
	}
	if _, ok := got["timeline"].([]any); !ok {
		t.Fatalf("expected timeline array present (dealDetailResponse.MarshalJSON fix), got %v", got["timeline"])
	}

	listReq := httptest.NewRequest(http.MethodGet, "/deals?pipeline_id="+pipelineID, nil)
	listReq = withDealWorkspace(listReq)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listW.Code, listW.Body.String())
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, d := range page.Data {
		if d["id"] == dealID {
			found = true
			if d[cfKey] != "alpha" {
				t.Fatalf("expected %s=alpha in list entry, got %v", cfKey, d[cfKey])
			}
		}
	}
	if !found {
		t.Fatal("expected created deal in list")
	}

	sortReq := httptest.NewRequest(http.MethodGet, "/deals?sort="+cfKey, nil)
	sortReq = withDealWorkspace(sortReq)
	sortW := httptest.NewRecorder()
	h.ServeHTTP(sortW, sortReq)
	if sortW.Code != http.StatusOK {
		t.Fatalf("sort by cf column status = %d, body=%s", sortW.Code, sortW.Body.String())
	}

	// Multi-sort mixed with a core column must still work: proves the
	// active-column merge didn't break dealOrderBy's comma-separated parsing.
	mixedSortReq := httptest.NewRequest(http.MethodGet, "/deals?sort=amount_minor,"+cfKey, nil)
	mixedSortReq = withDealWorkspace(mixedSortReq)
	mixedSortW := httptest.NewRecorder()
	h.ServeHTTP(mixedSortW, mixedSortReq)
	if mixedSortW.Code != http.StatusOK {
		t.Fatalf("mixed sort status = %d, body=%s", mixedSortW.Code, mixedSortW.Body.String())
	}
	var mixedPage struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(mixedSortW.Body.Bytes(), &mixedPage); err != nil {
		t.Fatalf("decode mixed sort list: %v", err)
	}
	mixedFound := false
	for _, d := range mixedPage.Data {
		if d["id"] == dealID {
			mixedFound = true
		}
	}
	if !mixedFound {
		t.Fatal("expected created deal present under the mixed core+custom sort")
	}

	updateBody, _ := json.Marshal(map[string]any{cfKey: "beta"})
	updateReq := httptest.NewRequest(http.MethodPatch, "/deals/"+dealID, bytes.NewReader(updateBody))
	updateReq = withDealWorkspace(updateReq)
	updateW := httptest.NewRecorder()
	h.ServeHTTP(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", updateW.Code, updateW.Body.String())
	}
	var updated map[string]any
	if err := json.Unmarshal(updateW.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated[cfKey] != "beta" {
		t.Fatalf("expected %s=beta after update, got %v", cfKey, updated[cfKey])
	}

	filterReq := httptest.NewRequest(http.MethodGet, "/deals?"+cfKey+"=beta", nil)
	filterReq = withDealWorkspace(filterReq)
	filterW := httptest.NewRecorder()
	h.ServeHTTP(filterW, filterReq)
	if filterW.Code != http.StatusOK {
		t.Fatalf("filter status = %d, body=%s", filterW.Code, filterW.Body.String())
	}
	var filterPage struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(filterW.Body.Bytes(), &filterPage); err != nil {
		t.Fatalf("decode filter list: %v", err)
	}
	if len(filterPage.Data) != 1 || filterPage.Data[0]["id"] != dealID {
		t.Fatalf("expected the cf_ filter to narrow the list to exactly the one deal, got %+v", filterPage.Data)
	}

	if _, err := customfields.Retire(cfCtx, db, field.ID); err != nil {
		t.Fatalf("retire: %v", err)
	}

	getReq2 := httptest.NewRequest(http.MethodGet, "/deals/"+dealID, nil)
	getReq2 = withDealWorkspace(getReq2)
	getW2 := httptest.NewRecorder()
	h.ServeHTTP(getW2, getReq2)
	if getW2.Code != http.StatusOK {
		t.Fatalf("get after retire status = %d, body=%s", getW2.Code, getW2.Body.String())
	}
	var got2 map[string]any
	if err := json.Unmarshal(getW2.Body.Bytes(), &got2); err != nil {
		t.Fatalf("decode get after retire: %v", err)
	}
	if _, exists := got2[cfKey]; exists {
		t.Fatalf("expected %s to be absent after retire, got %v", cfKey, got2[cfKey])
	}

	sortReq2 := httptest.NewRequest(http.MethodGet, "/deals?sort="+cfKey, nil)
	sortReq2 = withDealWorkspace(sortReq2)
	sortW2 := httptest.NewRecorder()
	h.ServeHTTP(sortW2, sortReq2)
	if sortW2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 sorting by a retired column, got %d: %s", sortW2.Code, sortW2.Body.String())
	}
	var sortProblem2 map[string]any
	_ = json.Unmarshal(sortW2.Body.Bytes(), &sortProblem2)
	if sortProblem2[fieldCode] != "sort_field_not_allowed" {
		t.Fatalf("expected code=sort_field_not_allowed, got %v", sortProblem2[fieldCode])
	}

	filterReq2 := httptest.NewRequest(http.MethodGet, "/deals?"+cfKey+"=beta", nil)
	filterReq2 = withDealWorkspace(filterReq2)
	filterW2 := httptest.NewRecorder()
	h.ServeHTTP(filterW2, filterReq2)
	if filterW2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 filtering by a retired column, got %d: %s", filterW2.Code, filterW2.Body.String())
	}
	var filterProblem2 map[string]any
	_ = json.Unmarshal(filterW2.Body.Bytes(), &filterProblem2)
	if filterProblem2[fieldCode] != "filter_field_not_allowed" {
		t.Fatalf("expected code=filter_field_not_allowed, got %v", filterProblem2[fieldCode])
	}
}
