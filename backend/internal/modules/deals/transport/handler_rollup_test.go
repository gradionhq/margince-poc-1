//go:build integration

package transport

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func openRollupTestDB(t *testing.T) *sql.DB {
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

func newRollupHandler(db *sql.DB) *PipelineHandler {
	return NewPipelineHandler(deals.NewPipelineStore(db), deals.NewStageStore(db), deals.NewRollupStore(db))
}

func withRollupWorkspace(r *http.Request, workspaceID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: workspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func seedRollupWorkspace(t *testing.T, db *sql.DB, tag string) (workspaceID, pipelineID, proposalStageID, negotiationStageID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if err := db.QueryRow(`INSERT INTO workspace (id, name, slug, base_currency)
		VALUES (uuidv7(),'t13-ws',$1,'EUR') RETURNING id`, "t13-ws-"+tag).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		workspaceID, "P-"+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Proposal', 1, 'open', 60) RETURNING id`,
		workspaceID, pipelineID).Scan(&proposalStageID); err != nil {
		t.Fatalf("seed proposal stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Negotiation', 2, 'open', 80) RETURNING id`,
		workspaceID, pipelineID).Scan(&negotiationStageID); err != nil {
		t.Fatalf("seed negotiation stage: %v", err)
	}
	return workspaceID, pipelineID, proposalStageID, negotiationStageID
}

func seedRollupDeal(t *testing.T, db *sql.DB, workspaceID, pipelineID, stageID string, amountMinor *int64, currency *string, status string) {
	t.Helper()
	var id string
	if err := db.QueryRow(`
		INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, amount_minor, currency, status,
		    closed_at, fx_rate_to_base, source, captured_by)
		VALUES (uuidv7(), $1::uuid, 'D', $2::uuid, $3::uuid, $4::bigint, $5::char(3), $6,
		    CASE WHEN $6 != 'open' THEN now() ELSE NULL END,
		    CASE WHEN $6 != 'open' AND $4::bigint IS NOT NULL THEN 1.0 ELSE NULL END,
		    'test', 'human:test')
		RETURNING id`,
		workspaceID, pipelineID, stageID, amountMinor, currency, status).Scan(&id); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
}

func amt(v int64) *int64   { return &v }
func cur(v string) *string { return &v }

func decodeRollup(t *testing.T, rec *httptest.ResponseRecorder) deals.PipelineRollup {
	t.Helper()
	var out deals.PipelineRollup
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	return out
}

func TestRollup_WorkedExample(t *testing.T) {
	db := openRollupTestDB(t)
	workspaceID, pipelineID, proposalID, negotiationID := seedRollupWorkspace(t, db, "worked")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, amt(10_000_000), cur("EUR"), "open")
	seedRollupDeal(t, db, workspaceID, pipelineID, negotiationID, amt(5_000_000), cur("USD"), "open")
	asOf := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1,'USD','EUR',0.92,$2)`, workspaceID, asOf); err != nil {
		t.Fatalf("seed rate: %v", err)
	}

	h := newRollupHandler(db)
	req := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	out := decodeRollup(t, rec)
	if out.UnweightedMinor != 14_600_000 {
		t.Errorf("unweighted = %d, want 14_600_000", out.UnweightedMinor)
	}
	if out.WeightedMinor != 9_680_000 {
		t.Errorf("weighted = %d, want 9_680_000", out.WeightedMinor)
	}
	var sumU, sumW int64
	for _, d := range out.Breakdown {
		sumU += d.BaseValueMinor
		sumW += d.WeightedValueMinor
	}
	if sumU != out.UnweightedMinor || sumW != out.WeightedMinor {
		t.Fatal("breakdown must sum exactly to the reported totals")
	}
}

func TestRollup_NullAmountContributesZeroWithMarker(t *testing.T) {
	db := openRollupTestDB(t)
	workspaceID, pipelineID, proposalID, _ := seedRollupWorkspace(t, db, "noamount")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, nil, nil, "open")

	h := newRollupHandler(db)
	req := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	out := decodeRollup(t, rec)
	if len(out.Breakdown) != 1 {
		t.Fatalf("breakdown len = %d, want 1 (deal must still appear, not be dropped)", len(out.Breakdown))
	}
	row := out.Breakdown[0]
	if row.BaseValueMinor != 0 || row.WeightedValueMinor != 0 || !row.NoAmount {
		t.Fatalf("row = %+v, want BaseValue=0 Weighted=0 NoAmount=true", row)
	}
	if out.UnweightedMinor != 0 || out.WeightedMinor != 0 {
		t.Fatalf("totals = %+v, want both 0", out)
	}
}

func TestRollup_ClosedDealExcluded(t *testing.T) {
	db := openRollupTestDB(t)
	workspaceID, pipelineID, proposalID, _ := seedRollupWorkspace(t, db, "closed")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, amt(1_000_000), cur("EUR"), "open")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, amt(9_000_000), cur("EUR"), "won")

	h := newRollupHandler(db)
	req := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := decodeRollup(t, rec)
	if len(out.Breakdown) != 1 {
		t.Fatalf("breakdown len = %d, want 1 (won deal excluded)", len(out.Breakdown))
	}
	if out.UnweightedMinor != 1_000_000 {
		t.Fatalf("unweighted = %d, want 1_000_000 (won deal's 9,000,000 must not count)", out.UnweightedMinor)
	}
}

func TestRollup_MissingFXRateReturns422(t *testing.T) {
	db := openRollupTestDB(t)
	workspaceID, pipelineID, proposalID, _ := seedRollupWorkspace(t, db, "missingfx")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, amt(1_000_000), cur("GBP"), "open")

	h := newRollupHandler(db)
	req := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "fx_rate_unavailable" {
		t.Fatalf("code = %v, want fx_rate_unavailable", body["code"])
	}
	details, _ := body["details"].(map[string]any)
	if details["currency"] != "GBP" || details["as_of"] == nil {
		t.Fatalf("details = %v, want currency=GBP + as_of populated", details)
	}
}

func TestRollup_MultiCurrencyNeverSumsNativeUnits(t *testing.T) {
	db := openRollupTestDB(t)
	workspaceID, pipelineID, proposalID, negotiationID := seedRollupWorkspace(t, db, "multicur")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, amt(1_000_000), cur("USD"), "open")
	seedRollupDeal(t, db, workspaceID, pipelineID, negotiationID, amt(1_000_000), cur("GBP"), "open")
	asOf := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1,'USD','EUR',0.90,$2), ($1,'GBP','EUR',1.15,$2)`,
		workspaceID, asOf); err != nil {
		t.Fatalf("seed rates: %v", err)
	}

	h := newRollupHandler(db)
	req := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := decodeRollup(t, rec)
	if out.UnweightedMinor != 2_050_000 {
		t.Fatalf("unweighted = %d, want 2_050_000 (each currency converts independently before summing)", out.UnweightedMinor)
	}
}

func TestRollup_LiveWinProbabilityRead(t *testing.T) {
	db := openRollupTestDB(t)
	workspaceID, pipelineID, proposalID, _ := seedRollupWorkspace(t, db, "live")
	seedRollupDeal(t, db, workspaceID, pipelineID, proposalID, amt(1_000_000), cur("EUR"), "open")

	h := newRollupHandler(db)
	req := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	first := decodeRollup(t, rec)
	if first.WeightedMinor != 600_000 {
		t.Fatalf("weighted = %d, want 600_000", first.WeightedMinor)
	}

	if _, err := db.Exec(`UPDATE stage SET win_probability=55 WHERE id=$1::uuid`, proposalID); err != nil {
		t.Fatalf("retune stage: %v", err)
	}

	req2 := withRollupWorkspace(httptest.NewRequest(http.MethodGet, "/pipelines/"+pipelineID+"/rollup", nil), workspaceID)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	second := decodeRollup(t, rec2)
	if second.WeightedMinor != 550_000 {
		t.Fatalf("weighted after retune = %d, want 550_000 (live read, not cached)", second.WeightedMinor)
	}
}
