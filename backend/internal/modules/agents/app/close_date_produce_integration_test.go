//go:build integration

package app_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

type stageSpec struct {
	name     string
	position int
	semantic string
	winProb  int
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

func seedOpenDealFull(t *testing.T, db *sql.DB, wsID, pipelineID, stageID string, expectedCloseDate *time.Time, forecastCategory *string, waitUntil *time.Time, lastActivityAt time.Time) string {
	t.Helper()
	var dealID string
	var forecast sql.NullString
	if forecastCategory != nil {
		forecast = sql.NullString{String: *forecastCategory, Valid: true}
	}
	var wait sql.NullTime
	if waitUntil != nil {
		wait = sql.NullTime{Time: *waitUntil, Valid: true}
	}
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, expected_close_date, forecast_category, last_activity_at, wait_until, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,'open',$5,$6,$7,$8,'fixture','agent:overnight') RETURNING id`,
		wsID, "agents-deal-"+ids.New(), pipelineID, stageID, expectedCloseDate, forecast, lastActivityAt, wait).Scan(&dealID); err != nil {
		t.Fatalf("seed open deal: %v", err)
	}
	return dealID
}

func seedClosedDealWithPastCloseDate(t *testing.T, db *sql.DB, wsID, pipelineID, stageID, status string, closeDate time.Time) string {
	t.Helper()
	var dealID string
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, expected_close_date, closed_at, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,$5,$6::date,$7,'fixture','agent:overnight') RETURNING id`,
		wsID, "agents-closed-"+ids.New(), pipelineID, stageID, status, closeDate.Format("2006-01-02"), closeDate).Scan(&dealID); err != nil {
		t.Fatalf("seed closed deal: %v", err)
	}
	return dealID
}

func assertDealClosesOnOrAfter(t *testing.T, db *sql.DB, dealID string, today time.Time) {
	t.Helper()
	var got sql.NullTime
	if err := db.QueryRow(`SELECT expected_close_date FROM deal WHERE id = $1::uuid`, dealID).Scan(&got); err != nil {
		t.Fatalf("query deal: %v", err)
	}
	if !got.Valid {
		t.Fatalf("deal %s expected_close_date is NULL, want >= %s", dealID, today.Format("2006-01-02"))
	}
	if got.Time.Before(today) {
		t.Fatalf("deal %s expected_close_date = %s, want >= %s", dealID, got.Time.Format("2006-01-02"), today.Format("2006-01-02"))
	}
}

func assertPendingApprovalActionType(t *testing.T, db *sql.DB, wsID, actionType string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT count(*) FROM approval_item WHERE workspace_id = $1::uuid AND action_type = $2 AND status = 'pending'`, wsID, actionType).Scan(&got); err != nil {
		t.Fatalf("count approval items: %v", err)
	}
	if got != want {
		t.Fatalf("pending approval items for %s = %d, want %d", actionType, got, want)
	}
}

func TestRunPass_CloseDateHygiene_FullSweep_InvariantHoldsAcrossEveryTier(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := seedPipeline(t, db, wsID, []stageSpec{
		{name: "Discovery", position: 1, semantic: "open", winProb: 20},
		{name: "Negotiation", position: 2, semantic: "open", winProb: 60},
		{name: "Won", position: 3, semantic: "won", winProb: 100},
	})
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	overdue := now.AddDate(0, 0, -12)
	quiet := now.AddDate(0, 0, -75)
	waitUntil := now.AddDate(0, 0, 5)
	healthyDate := now.AddDate(0, 0, 30)

	autoApplyID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].id, &overdue, nil, nil, now.AddDate(0, 0, -3))
	provisionalLateStageID := seedOpenDealFull(t, db, wsID, pipelineID, stages[1].id, &overdue, nil, nil, now.AddDate(0, 0, -3))
	missingID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].id, nil, nil, nil, now.AddDate(0, 0, -3))
	downgradeID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].id, &overdue, nil, nil, quiet)
	waitSuppressedID := seedOpenDealFull(t, db, wsID, pipelineID, stages[1].id, &overdue, nil, &waitUntil, quiet)
	wonID := seedClosedDealWithPastCloseDate(t, db, wsID, pipelineID, stages[2].id, "won", overdue)
	healthyID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].id, &healthyDate, nil, nil, now.AddDate(0, 0, -1))

	reader := adapters.NewSQLDealReader(db)
	dealStore := deals.NewDealStore(db)
	effector := adapters.NewCloseDateEffector(dealStore)
	repo := crmapprovals.NewRepository()
	produce := app.NewCloseDateProduce(reader, func() time.Time { return now })

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: domain.AssembledView{WorkspaceID: wsID}},
		Since:       now.Add(-24 * time.Hour),
		Produce:     produce,
		Stage:       crmapprovals.Stage,
		Repo:        repo,
		Effector:    effector,
		Emitter:     &spyEmitter{},
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("RunPass: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if result.State != domain.RunNormal {
		t.Fatalf("state = %v, want RunNormal", result.State)
	}

	var pastCount int
	if err := db.QueryRow(`SELECT count(*) FROM deal WHERE workspace_id = $1::uuid AND status = 'open' AND expected_close_date < $2::date`, wsID, now.Format("2006-01-02")).Scan(&pastCount); err != nil {
		t.Fatalf("count past-date open deals: %v", err)
	}
	if pastCount != 0 {
		t.Fatalf("OVN-AC-1 violated: %d open deals carry a past close date after the run", pastCount)
	}

	assertDealClosesOnOrAfter(t, db, autoApplyID, now)
	assertPendingApprovalActionType(t, db, wsID, "overnight.close-date-confirm-request", 3)
	assertPendingApprovalActionType(t, db, wsID, "overnight.close-date-downgrade-review", 1)

	wonAfter, _ := dealStore.GetAny(context.Background(), wonID, wsID)
	if wonAfter.ExpectedCloseDate == nil || !wonAfter.ExpectedCloseDate.Before(now) {
		t.Fatalf("won deal's close date must be untouched (still overdue), got %v", wonAfter.ExpectedCloseDate)
	}

	waitAfter, _ := dealStore.Get(context.Background(), waitSuppressedID, wsID)
	if waitAfter.ExpectedCloseDate == nil || waitAfter.ExpectedCloseDate.Before(now) {
		t.Fatalf("wait-suppressed deal must hold a non-past provisional date, got %v", waitAfter.ExpectedCloseDate)
	}

	healthyAfter, _ := dealStore.Get(context.Background(), healthyID, wsID)
	if healthyAfter.ExpectedCloseDate == nil || healthyAfter.ExpectedCloseDate.Format("2006-01-02") != healthyDate.Format("2006-01-02") {
		t.Fatalf("healthy (unflagged) deal must be left exactly as seeded, got %v", healthyAfter.ExpectedCloseDate)
	}

	_ = provisionalLateStageID
	_ = missingID
	_ = downgradeID
}
