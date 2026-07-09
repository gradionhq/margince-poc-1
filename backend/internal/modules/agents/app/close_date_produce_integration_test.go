//go:build integration

package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/agents/agentstest"
	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// agentstest.SeedPipeline/agentstest.StageSpec (used below) are the pipeline
// seed fixture shared with agents/adapters' integration tests — see that
// package's doc comment for why it is a separate, non-`_test.go` package
// rather than a local helper.

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

// seedClosedDealWithPastCloseDate seeds a won or lost deal (lostReason is
// required by the deal_lost_reason DB constraint when status = "lost", and
// must be nil otherwise).
func seedClosedDealWithPastCloseDate(t *testing.T, db *sql.DB, wsID, pipelineID, stageID, status string, closeDate time.Time, lostReason *string) string {
	t.Helper()
	var dealID string
	var reason sql.NullString
	if lostReason != nil {
		reason = sql.NullString{String: *lostReason, Valid: true}
	}
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, status, expected_close_date, closed_at, lost_reason, source, captured_by)
		VALUES ($1::uuid,$2,$3::uuid,$4::uuid,$5,$6::date,$7,$8,'fixture','agent:overnight') RETURNING id`,
		wsID, "agents-closed-"+ids.New(), pipelineID, stageID, status, closeDate.Format("2006-01-02"), closeDate, reason).Scan(&dealID); err != nil {
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

// assertApprovalPayloadReason reads the persisted approval_item.payload for
// the one pending item matching (actionType, dealID) and asserts its
// "reason"/"message" fields — proving the review-item's framing (e.g. "gone
// quiet" vs "confirm the date"), not just its action_type routing.
func assertApprovalPayloadReason(t *testing.T, db *sql.DB, wsID, actionType, dealID, wantReason, wantMessageSubstring string) {
	t.Helper()
	var raw []byte
	if err := db.QueryRow(`SELECT payload FROM approval_item WHERE workspace_id = $1::uuid AND action_type = $2 AND status = 'pending' AND payload->>'deal_id' = $3`,
		wsID, actionType, dealID).Scan(&raw); err != nil {
		t.Fatalf("query approval item payload for %s / deal %s: %v", actionType, dealID, err)
	}
	var payload struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal approval item payload: %v", err)
	}
	if payload.Reason != wantReason {
		t.Fatalf("payload reason for %s / deal %s = %q, want %q", actionType, dealID, payload.Reason, wantReason)
	}
	if !strings.Contains(payload.Message, wantMessageSubstring) {
		t.Fatalf("payload message for %s / deal %s = %q, want substring %q", actionType, dealID, payload.Message, wantMessageSubstring)
	}
}

func TestRunPass_CloseDateHygiene_FullSweep_InvariantHoldsAcrossEveryTier(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := agentstest.SeedPipeline(t, db, wsID, []agentstest.StageSpec{
		{Name: "Discovery", Position: 1, Semantic: "open", WinProb: 20},
		{Name: "Negotiation", Position: 2, Semantic: "open", WinProb: 60},
		{Name: "Won", Position: 3, Semantic: "won", WinProb: 100},
		{Name: "Lost", Position: 4, Semantic: "lost", WinProb: 0},
	})
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	overdue := now.AddDate(0, 0, -12)
	quiet := now.AddDate(0, 0, -75)
	waitUntil := now.AddDate(0, 0, 5)
	healthyDate := now.AddDate(0, 0, 30)

	autoApplyID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].ID, &overdue, nil, nil, now.AddDate(0, 0, -3))
	provisionalLateStageID := seedOpenDealFull(t, db, wsID, pipelineID, stages[1].ID, &overdue, nil, nil, now.AddDate(0, 0, -3))
	missingID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].ID, nil, nil, nil, now.AddDate(0, 0, -3))
	downgradeID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].ID, &overdue, nil, nil, quiet)
	waitSuppressedID := seedOpenDealFull(t, db, wsID, pipelineID, stages[1].ID, &overdue, nil, &waitUntil, quiet)
	wonID := seedClosedDealWithPastCloseDate(t, db, wsID, pipelineID, stages[2].ID, "won", overdue, nil)
	lostReason := "budget cut"
	lostID := seedClosedDealWithPastCloseDate(t, db, wsID, pipelineID, stages[3].ID, "lost", overdue, &lostReason)
	healthyID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].ID, &healthyDate, nil, nil, now.AddDate(0, 0, -1))

	// PROVISIONAL_CONFIRM (rep-set forecast_category="commit"): Discovery
	// stage (early, <50%, so NOT late_stage), otherwise AUTO_APPLY-eligible
	// (clear-overdue, active) — the explicit rep-set category alone must
	// still route this to PROVISIONAL_CONFIRM via InForecastCommit's
	// rep-override branch, distinct from the >=50-probability default this
	// suite already covers via provisionalLateStageID.
	commitOverride := "commit"
	commitOverrideID := seedOpenDealFull(t, db, wsID, pipelineID, stages[0].ID, &overdue, &commitOverride, nil, now.AddDate(0, 0, -3))

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
	assertDealClosesOnOrAfter(t, db, commitOverrideID, now)
	assertPendingApprovalActionType(t, db, wsID, "overnight.close-date-confirm-request", 4) // late_stage + missing + wait-suppressed(late_stage) + commit-override cases
	assertPendingApprovalActionType(t, db, wsID, "overnight.close-date-downgrade-review", 1)

	// The quiet deal's review item must say "gone quiet", never re-framed as
	// a routine confirm-the-date ask — this is the DOWNGRADE_AND_REVIEW vs
	// PROVISIONAL_CONFIRM distinction the spec calls out as load-bearing.
	assertApprovalPayloadReason(t, db, wsID, "overnight.close-date-downgrade-review", downgradeID, "quiet", "gone quiet")
	assertApprovalPayloadReason(t, db, wsID, "overnight.close-date-confirm-request", provisionalLateStageID, "provisional_confirm", "Confirm the real close date")

	wonAfter, _ := dealStore.GetAny(context.Background(), wonID, wsID)
	if wonAfter.ExpectedCloseDate == nil || !wonAfter.ExpectedCloseDate.Before(now) {
		t.Fatalf("won deal's close date must be untouched (still overdue), got %v", wonAfter.ExpectedCloseDate)
	}

	lostAfter, _ := dealStore.GetAny(context.Background(), lostID, wsID)
	if lostAfter.ExpectedCloseDate == nil || !lostAfter.ExpectedCloseDate.Before(now) {
		t.Fatalf("lost deal's close date must be untouched (still overdue), got %v", lostAfter.ExpectedCloseDate)
	}

	waitAfter, _ := dealStore.Get(context.Background(), waitSuppressedID, wsID)
	if waitAfter.ExpectedCloseDate == nil || waitAfter.ExpectedCloseDate.Before(now) {
		t.Fatalf("wait-suppressed deal must hold a non-past provisional date, got %v", waitAfter.ExpectedCloseDate)
	}

	healthyAfter, _ := dealStore.Get(context.Background(), healthyID, wsID)
	if healthyAfter.ExpectedCloseDate == nil || healthyAfter.ExpectedCloseDate.Format("2006-01-02") != healthyDate.Format("2006-01-02") {
		t.Fatalf("healthy (unflagged) deal must be left exactly as seeded, got %v", healthyAfter.ExpectedCloseDate)
	}

	_ = missingID
}
