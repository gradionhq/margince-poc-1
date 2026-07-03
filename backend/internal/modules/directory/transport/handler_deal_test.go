//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
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
	h := NewDealHandler(crmcore.NewDealStore(db))

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
	var created crmcore.Deal
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
	h := NewDealHandler(crmcore.NewDealStore(db))

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
	var first crmcore.Deal
	_ = json.Unmarshal(w1.Body.Bytes(), &first)

	req2 := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(b))
	req2 = withDealWorkspace(req2)
	req2.Header.Set("Idempotency-Key", "replay-key-1")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	var second crmcore.Deal
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
	h := NewDealHandler(crmcore.NewDealStore(db))

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
