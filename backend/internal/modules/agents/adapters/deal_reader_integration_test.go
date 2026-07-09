//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/adapters"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type stageSpec struct {
	name     string
	position int
	semantic string
	winProb  int
}

func seedWorkspace(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "agents-"+wsID, "agents-"+wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return wsID
}

func seedPipeline(t *testing.T, db *sql.DB, wsID string, stages []stageSpec) (string, []struct{ id string }) {
	t.Helper()
	var pipelineID string
	if err := db.QueryRow(`INSERT INTO pipeline (workspace_id, name) VALUES ($1::uuid,$2) RETURNING id`, wsID, "agents-pipeline-"+ids.New()).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	out := make([]struct{ id string }, 0, len(stages))
	for _, s := range stages {
		var id string
		if err := db.QueryRow(`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic, win_probability)
			VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6) RETURNING id`, wsID, pipelineID, s.name, s.position, s.semantic, s.winProb).Scan(&id); err != nil {
			t.Fatalf("seed stage %q: %v", s.name, err)
		}
		out = append(out, struct{ id string }{id: id})
	}
	return pipelineID, out
}

func seedOpenDeal(t *testing.T, db *sql.DB, wsID, pipelineID, stageID string, expectedCloseDate *time.Time) string {
	t.Helper()
	var dealID string
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, expected_close_date, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,'open',$5,'fixture','agent:overnight') RETURNING id`,
		wsID, "agents-deal-"+ids.New(), pipelineID, stageID, expectedCloseDate).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return dealID
}

func seedWonDealWithHistory(t *testing.T, db *sql.DB, wsID, pipelineID string, stages []struct{ id string }, days int) string {
	t.Helper()
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	startAt := now.AddDate(0, 0, -days)
	var dealID string
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, closed_at, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,'won',$5,'fixture','agent:overnight') RETURNING id`,
		wsID, "agents-won-"+ids.New(), pipelineID, stages[len(stages)-1].id, now).Scan(&dealID); err != nil {
		t.Fatalf("seed won deal: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id, changed_by, occurred_at)
		VALUES ($1::uuid,$2::uuid,NULL,$3::uuid,'agent:overnight',$4)`,
		wsID, dealID, stages[0].id, startAt); err != nil {
		t.Fatalf("seed history start: %v", err)
	}
	for i := 0; i < len(stages)-1; i++ {
		occurredAt := now
		if _, err := db.Exec(`INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id, changed_by, occurred_at)
			VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,'agent:overnight',$5)`,
			wsID, dealID, stages[i].id, stages[i+1].id, occurredAt); err != nil {
			t.Fatalf("seed history %d: %v", i, err)
		}
	}
	return dealID
}

func TestSQLDealReader_ListOpenDeals_ResolvesWinProbabilityAndRemainingStages(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := seedPipeline(t, db, wsID, []stageSpec{
		{name: "Discovery", position: 1, semantic: "open", winProb: 20},
		{name: "Negotiation", position: 2, semantic: "open", winProb: 60},
		{name: "Won", position: 3, semantic: "won", winProb: 100},
	})
	dealID := seedOpenDeal(t, db, wsID, pipelineID, stages[0].id, nil)

	r := adapters.NewSQLDealReader(db)
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	snaps, err := r.ListOpenDeals(context.Background(), wsID, now)
	if err != nil {
		t.Fatalf("ListOpenDeals: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 open deal, got %d", len(snaps))
	}
	s := snaps[0]
	if s.DealID != dealID || s.WinProbability != 20 {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
	if s.RemainingOpenStages != 2 {
		t.Fatalf("RemainingOpenStages = %d, want 2", s.RemainingOpenStages)
	}
}

func TestSQLDealReader_PipelineWonVelocity_FallbackBelowMinHistory(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, _ := seedPipeline(t, db, wsID, []stageSpec{{name: "Open", position: 1, semantic: "open", winProb: 50}})

	r := adapters.NewSQLDealReader(db)
	_, wonCount, err := r.PipelineWonVelocity(context.Background(), wsID, pipelineID)
	if err != nil {
		t.Fatalf("PipelineWonVelocity: %v", err)
	}
	if wonCount != 0 {
		t.Fatalf("wonCount = %d, want 0", wonCount)
	}
}

func TestSQLDealReader_PipelineWonVelocity_MedianOverWonDealHistory(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := seedPipeline(t, db, wsID, []stageSpec{
		{name: "Discovery", position: 1, semantic: "open", winProb: 20},
		{name: "Won", position: 2, semantic: "won", winProb: 100},
	})
	seedWonDealWithHistory(t, db, wsID, pipelineID, stages, 10)

	r := adapters.NewSQLDealReader(db)
	median, wonCount, err := r.PipelineWonVelocity(context.Background(), wsID, pipelineID)
	if err != nil {
		t.Fatalf("PipelineWonVelocity: %v", err)
	}
	if wonCount != 1 || median != 10 {
		t.Fatalf("median=%d wonCount=%d, want median=10 wonCount=1", median, wonCount)
	}
}
