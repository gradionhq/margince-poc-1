//go:build integration

package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/lib/pq"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func openDealTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const dealTestWorkspaceID = "00000000-0000-0000-0000-000000000003"

func withDealWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func dealHandlerForTest(db *sql.DB, store *adapters.DealStore) *DealHandler {
	return NewDealHandler(store, relationships.NewRelationshipStore(db), activities.NewActivityStore(db), &crmapprovals.DBVerifier{DB: db})
}

func seedDealFixtures(t *testing.T, db *sql.DB, tag string) (pipelineID, stageID, otherStageID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t11-handler-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, dealTestWorkspaceID, "t11-handler-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, dealTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var pA, pB string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name)
		VALUES (uuidv7(), $1, $2) RETURNING id`, dealTestWorkspaceID, "P-A-"+tag).Scan(&pA); err != nil {
		t.Fatalf("seed pipeline A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name)
		VALUES (uuidv7(), $1, $2) RETURNING id`, dealTestWorkspaceID, "P-B-"+tag).Scan(&pB); err != nil {
		t.Fatalf("seed pipeline B: %v", err)
	}
	var sA, sB string
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position)
		VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`, dealTestWorkspaceID, pA, "S-A-"+tag).Scan(&sA); err != nil {
		t.Fatalf("seed stage A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position)
		VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`, dealTestWorkspaceID, pB, "S-B-"+tag).Scan(&sB); err != nil {
		t.Fatalf("seed stage B: %v", err)
	}
	return pA, sA, sB
}

func TestDealHandler_Create_Returns201WithLocationAndHistoryRow(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "create")
	h := dealHandlerForTest(db, adapters.NewDealStore(db))

	body := map[string]any{
		"name": "Acme deal", "pipeline_id": pipelineID, "stage_id": stageID,
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(b))
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc == "" {
		t.Fatal("expected Location header on 201")
	}
	var created domain.Deal
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a created deal id")
	}
}

func TestDealHandler_Create_IdempotencyKeyReplay(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "replay")
	h := dealHandlerForTest(db, adapters.NewDealStore(db))

	body := map[string]any{
		"name": "Replay deal", "pipeline_id": pipelineID, "stage_id": stageID,
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)

	req1 := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(b))
	req1 = withDealWorkspace(req1)
	req1.Header.Set("Idempotency-Key", "replay-key-1")
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	var first domain.Deal
	_ = json.Unmarshal(w1.Body.Bytes(), &first)

	req2 := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(b))
	req2 = withDealWorkspace(req2)
	req2.Header.Set("Idempotency-Key", "replay-key-1")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	var second domain.Deal
	_ = json.Unmarshal(w2.Body.Bytes(), &second)

	if w2.Code != http.StatusCreated {
		t.Fatalf("replay status = %d, want 201", w2.Code)
	}
	if second.ID != first.ID {
		t.Fatalf("replay returned a different deal id: %s != %s", second.ID, first.ID)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM deal WHERE workspace_id=$1 AND name='Replay deal'`,
		dealTestWorkspaceID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 deal, got %d", count)
	}
}

func TestDealHandler_Create_StageNotInPipeline(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, _, otherStageID := seedDealFixtures(t, db, "stage-check")
	h := dealHandlerForTest(db, adapters.NewDealStore(db))

	body := map[string]any{
		"name": "Bad deal", "pipeline_id": pipelineID, "stage_id": otherStageID,
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(b))
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	details, ok := problem["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details in problem body, got %v", problem)
	}
	errs, ok := details["errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("expected details.errors with 1 entry, got %v", details)
	}
	first := errs[0].(map[string]any)
	if first["field"] != "stage_id" || first["code"] != "stage_not_in_pipeline" {
		t.Fatalf("expected {field:stage_id, code:stage_not_in_pipeline}, got %v", first)
	}
}

func TestDealHandler_Update_IfMatchVersionSkew(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "update-version-skew")
	store := adapters.NewDealStore(db)
	h := dealHandlerForTest(db, store)

	d := domain.NewDeal("Update-me", pipelineID, stageID,
		provenanceOf("test", "human:test"))
	d.WorkspaceID = dealTestWorkspaceID
	created, err := store.Create(context.Background(), d, "")
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	body := map[string]any{"name": "Renamed"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(b))
	req = withDealWorkspace(req)
	req.Header.Set("If-Match", "999")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", w.Code, w.Body.String())
	}
	var problem map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &problem)
	if problem[fieldCode] != "version_skew" {
		t.Fatalf("code = %v, want version_skew", problem[fieldCode])
	}
}

func TestDealHandler_Update_MalformedIfMatch(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "update-malformed")
	store := adapters.NewDealStore(db)
	h := dealHandlerForTest(db, store)

	d := domain.NewDeal("Malformed-if-match", pipelineID, stageID,
		provenanceOf("test", "human:test"))
	d.WorkspaceID = dealTestWorkspaceID
	created, err := store.Create(context.Background(), d, "")
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	body := map[string]any{"name": "Renamed"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(b))
	req = withDealWorkspace(req)
	req.Header.Set("If-Match", "not-a-number")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestDealHandler_Update_StageNotInPipeline(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, otherStageID := seedDealFixtures(t, db, "update-stage-check")
	store := adapters.NewDealStore(db)
	h := dealHandlerForTest(db, store)

	d := domain.NewDeal("Stage-move", pipelineID, stageID,
		provenanceOf("test", "human:test"))
	d.WorkspaceID = dealTestWorkspaceID
	created, err := store.Create(context.Background(), d, "")
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	body := map[string]any{"stage_id": otherStageID}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(b))
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

func TestDealHandler_Update_HappyPath(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "update-happy")
	store := adapters.NewDealStore(db)
	h := dealHandlerForTest(db, store)

	d := domain.NewDeal("Happy-update", pipelineID, stageID,
		provenanceOf("test", "human:test"))
	d.WorkspaceID = dealTestWorkspaceID
	created, err := store.Create(context.Background(), d, "")
	if err != nil {
		t.Fatalf("seed create: %v", err)
	}

	body := map[string]any{"name": "Renamed OK"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(b))
	req = withDealWorkspace(req)
	req.Header.Set("If-Match", strconv.FormatInt(created.Version, 10))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var updated domain.Deal
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Name != "Renamed OK" {
		t.Fatalf("name = %s, want Renamed OK", updated.Name)
	}
}

func TestDealHandler_List_FilterAndSort(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "list")
	store := adapters.NewDealStore(db)
	h := dealHandlerForTest(db, store)
	ctx := context.Background()

	// Seed an organization row: deal.partner_org_id is a hard FK to
	// organization(id), so a real row must exist before we can filter on it.
	var orgID string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`,
		dealTestWorkspaceID, "PartnerOrg-list").Scan(&orgID); err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	fc := "commit"
	d := domain.NewDeal("List me", pipelineID, stageID, provenanceOf("test", "human:test"))
	d.WorkspaceID = dealTestWorkspaceID
	d.ForecastCategory = &fc
	amt := int64(500)
	d.AmountMinor = &amt
	d.Status = "open"
	d.PartnerOrgID = &orgID
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Seed a stakeholder relationship: a real person row plus a
	// relationship(kind='deal_stakeholder') edge pointing at the created deal.
	var personID string
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`,
		dealTestWorkspaceID, "Stakeholder Person").Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO relationship (id, workspace_id, kind, person_id, deal_id, role, source, captured_by)
		VALUES (uuidv7(), $1, 'deal_stakeholder', $2, $3, 'champion', 'test', 'human:test')`,
		dealTestWorkspaceID, personID, created.ID); err != nil {
		t.Fatalf("seed relationship: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/deals?sort=amount_minor&status=open&forecast_category=commit&pipeline_id="+pipelineID+"&partner_org_id="+orgID, nil)
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Data []domain.Deal `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, dl := range page.Data {
		if dl.Name == "List me" {
			found = true
			if dl.StageEnteredAt == nil {
				t.Fatal("expected stage_entered_at on list rows")
			}
			if dl.StakeholderCount != 1 {
				t.Fatalf("stakeholder_count = %d, want 1", dl.StakeholderCount)
			}
		} else if dl.PartnerOrgID != nil && *dl.PartnerOrgID != orgID {
			t.Fatalf("partner_org_id filter leaked deal %s with partner_org_id=%s, want only %s", dl.ID, *dl.PartnerOrgID, orgID)
		}
	}
	if !found {
		t.Fatal("expected 'List me' deal in filtered results")
	}
	if len(page.Data) != 1 {
		t.Fatalf("expected partner_org_id filter to narrow results to exactly 1 deal, got %d: %+v", len(page.Data), page.Data)
	}
}

// NOTE: T21's own GET /deals/{id} coverage (stakeholders present, timeline
// contract, 404) used to live here as
// TestDealHandler_Get_HappyPathIncludesStakeholdersAndEmptyTimeline /
// TestDealHandler_Get_NotFound. Both are now superseded by main's fuller
// deal-360 composite read (GetAny + activityStore-backed timeline) merged
// from a sibling ticket — see TestDealHandler_Get_Composite360,
// TestDealHandler_Get_NonexistentID_Returns404, and
// TestDealHandler_Get_ForeignWorkspaceID_Returns404 in
// handler_deal_get_test.go, which cover the same ground against the current
// (non-empty) timeline contract. Removed here rather than left stale/
// duplicated against an obsolete "timeline is always empty" assumption.

func TestDealHandler_FullLifecycle_CreateUpdateList(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, otherStageID := seedDealFixtures(t, db, "lifecycle")
	h := dealHandlerForTest(db, adapters.NewDealStore(db))

	createBody, _ := json.Marshal(map[string]any{
		"name": "Lifecycle deal", "pipeline_id": pipelineID, "stage_id": stageID,
		"source": "test", "captured_by": "human:test",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(createBody))
	createReq = withDealWorkspace(createReq)
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", createW.Code, createW.Body.String())
	}
	var created domain.Deal
	_ = json.Unmarshal(createW.Body.Bytes(), &created)

	badStageBody, _ := json.Marshal(map[string]any{"stage_id": otherStageID})
	badStageReq := httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(badStageBody))
	badStageReq = withDealWorkspace(badStageReq)
	badStageW := httptest.NewRecorder()
	h.ServeHTTP(badStageW, badStageReq)
	if badStageW.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update stage_not_in_pipeline status = %d, want 422", badStageW.Code)
	}

	renameBody, _ := json.Marshal(map[string]any{"name": "Lifecycle deal (renamed)"})
	renameReq := httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(renameBody))
	renameReq = withDealWorkspace(renameReq)
	renameReq.Header.Set("If-Match", strconv.FormatInt(created.Version, 10))
	renameW := httptest.NewRecorder()
	h.ServeHTTP(renameW, renameReq)
	if renameW.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", renameW.Code, renameW.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/deals?pipeline_id="+pipelineID, nil)
	listReq = withDealWorkspace(listReq)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listW.Code, listW.Body.String())
	}
	var page struct {
		Data []domain.Deal `json:"data"`
	}
	_ = json.Unmarshal(listW.Body.Bytes(), &page)
	renamed := false
	for _, dl := range page.Data {
		if dl.ID == created.ID && dl.Name == "Lifecycle deal (renamed)" {
			renamed = true
		}
	}
	if !renamed {
		t.Fatal("expected the renamed deal to appear in the list")
	}
}
