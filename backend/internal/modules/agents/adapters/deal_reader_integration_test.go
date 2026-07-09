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
	"github.com/gradionhq/margince/backend/internal/modules/agents/agentstest"
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

func seedWorkspace(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "agents-"+wsID, "agents-"+wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return wsID
}

// agentstest.SeedPipeline/agentstest.StageSpec (used below) are the pipeline
// seed fixture shared with agents/app's integration tests — see that
// package's doc comment for why it is a separate, non-`_test.go` package
// rather than a local helper.

// seedOpenDeal seeds a bare open deal with no expected_close_date — every
// caller in this package only needs a plain open deal to read/write against;
// app.seedOpenDealFull is the fixture that varies expected_close_date/
// forecast_category/wait_until for the close-date-hygiene decision scenarios.
func seedOpenDeal(t *testing.T, db *sql.DB, wsID, pipelineID, stageID string) string {
	t.Helper()
	var dealID string
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,'open','fixture','agent:overnight') RETURNING id`,
		wsID, "agents-deal-"+ids.New(), pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return dealID
}

func seedWonDealWithHistory(t *testing.T, db *sql.DB, wsID, pipelineID string, stages []struct{ ID string }, days int) string {
	t.Helper()
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	startAt := now.AddDate(0, 0, -days)
	var dealID string
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, closed_at, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,'won',$5,'fixture','agent:overnight') RETURNING id`,
		wsID, "agents-won-"+ids.New(), pipelineID, stages[len(stages)-1].ID, now).Scan(&dealID); err != nil {
		t.Fatalf("seed won deal: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id, changed_by, occurred_at)
		VALUES ($1::uuid,$2::uuid,NULL,$3::uuid,'agent:overnight',$4)`,
		wsID, dealID, stages[0].ID, startAt); err != nil {
		t.Fatalf("seed history start: %v", err)
	}
	for i := 0; i < len(stages)-1; i++ {
		occurredAt := now
		if _, err := db.Exec(`INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id, changed_by, occurred_at)
			VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,'agent:overnight',$5)`,
			wsID, dealID, stages[i].ID, stages[i+1].ID, occurredAt); err != nil {
			t.Fatalf("seed history %d: %v", i, err)
		}
	}
	return dealID
}

func TestSQLDealReader_ListOpenDeals_ResolvesWinProbabilityAndRemainingStages(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := agentstest.SeedPipeline(t, db, wsID, []agentstest.StageSpec{
		{Name: "Discovery", Position: 1, Semantic: "open", WinProb: 20},
		{Name: "Negotiation", Position: 2, Semantic: "open", WinProb: 60},
		{Name: "Won", Position: 3, Semantic: "won", WinProb: 100},
	})
	dealID := seedOpenDeal(t, db, wsID, pipelineID, stages[0].ID)

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
	pipelineID, _ := agentstest.SeedPipeline(t, db, wsID, []agentstest.StageSpec{{Name: "Open", Position: 1, Semantic: "open", WinProb: 50}})

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
	pipelineID, stages := agentstest.SeedPipeline(t, db, wsID, []agentstest.StageSpec{
		{Name: "Discovery", Position: 1, Semantic: "open", WinProb: 20},
		{Name: "Won", Position: 2, Semantic: "won", WinProb: 100},
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
