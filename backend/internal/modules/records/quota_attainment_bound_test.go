//go:build integration

package records_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// seedQuota creates a quota via the store and returns its id.
func seedQuota(t *testing.T, store *records.QuotaStore, ws string, ownerID, teamID *string, periodStart, periodEnd time.Time, targetMinor int64, currency string) string {
	t.Helper()
	q, err := store.Create(pgtest.AppCtx(ws), records.Quota{
		WorkspaceID: ws,
		OwnerID:     ownerID,
		TeamID:      teamID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TargetMinor: targetMinor,
		Currency:    currency,
	})
	if err != nil {
		t.Fatalf("seed quota: %v", err)
	}
	return q.ID
}

// seedQuotaWonDeal inserts a won deal scoped to ownerID with the given currency and fx_rate_to_base.
// amount_minor_base is a GENERATED column (round(amount_minor * fx_rate_to_base)) — set fx=1.0 for identity.
func seedQuotaWonDeal(t *testing.T, db *sql.DB, ws, pipelineID, stageID, ownerID string, amountMinor int64, currency, fxRateToBase string, closedAt time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, owner_id, amount_minor,
		                    currency, fx_rate_to_base, status, closed_at, source, captured_by)
		 VALUES ($1,'QuotaWonDeal',$2,$3,$4,$5,$6,$7,'won',$8,'api','human:t')`,
		ws, pipelineID, stageID, ownerID, amountMinor, currency, fxRateToBase, closedAt,
	); err != nil {
		t.Fatalf("seed quota won deal: %v", err)
	}
}

func TestQuotaStore_Attainment_GoldenNumber(t *testing.T) {
	// RD-AC-3: spec's worked example — 313,872.00 EUR won vs 280,000.00 EUR target.
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db) // base_currency=EUR
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	ownerID := seedUser(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)

	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	inPeriod := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	qID := seedQuota(t, store, ws, &ownerID, nil, periodStart, periodEnd, 28000000, "EUR")

	// Two EUR deals totaling 31,387,200 minor = 313,872.00 EUR.
	// fx_rate_to_base='1.0000000000' → amount_minor_base = amount_minor (GENERATED column identity).
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, ownerID, 18000000, "EUR", "1.0000000000", inPeriod)
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, ownerID, 13387200, "EUR", "1.0000000000", inPeriod)

	att, err := store.Attainment(ctx, qID, ws)
	if err != nil {
		t.Fatalf("Attainment: %v", err)
	}

	if att.ClosedWonMinor != 31387200 {
		t.Errorf("closed_won_minor = %d, want 31387200", att.ClosedWonMinor)
	}
	if att.GapMinor != 3387200 {
		t.Errorf("gap_minor = %d, want +3387200 (+33,872.00 EUR)", att.GapMinor)
	}
	wantPct := 31387200.0 / 28000000.0 * 100
	if diff := att.AttainmentPct - wantPct; diff < -0.01 || diff > 0.01 {
		t.Errorf("attainment_pct = %v, want ≈%v", att.AttainmentPct, wantPct)
	}
	if att.Band != "met" {
		t.Errorf("band = %q, want met", att.Band)
	}
	if len(att.ContributingDeals) != 2 {
		t.Errorf("contributing_deals len = %d, want 2", len(att.ContributingDeals))
	}
	var sumContrib int64
	for _, d := range att.ContributingDeals {
		sumContrib += d.BaseValueMinor
	}
	if sumContrib != att.ClosedWonMinor {
		t.Errorf("contributing_deals sum = %d, must equal closed_won_minor %d", sumContrib, att.ClosedWonMinor)
	}
	if att.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", att.Currency)
	}
	if att.QuotaID != qID {
		t.Errorf("quota_id = %q, want %q", att.QuotaID, qID)
	}
}

func TestQuotaStore_Attainment_OwnerScoping(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	owner := seedUser(t, db, ws)
	other := seedUser(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)

	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	inPeriod := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	qID := seedQuota(t, store, ws, &owner, nil, periodStart, periodEnd, 10000000, "EUR")
	// Owner's deal counts; other user's deal must not.
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, owner, 5000000, "EUR", "1.0000000000", inPeriod)
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, other, 9000000, "EUR", "1.0000000000", inPeriod)

	att, err := store.Attainment(ctx, qID, ws)
	if err != nil {
		t.Fatalf("Attainment (owner-scoped): %v", err)
	}
	if att.ClosedWonMinor != 5000000 {
		t.Errorf("owner-scoped closed_won_minor = %d, want 5000000 (other's deal excluded)", att.ClosedWonMinor)
	}
	if len(att.ContributingDeals) != 1 {
		t.Errorf("contributing_deals len = %d, want 1", len(att.ContributingDeals))
	}
}

func TestQuotaStore_Attainment_TeamScoping(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	member1 := seedUser(t, db, ws)
	member2 := seedUser(t, db, ws)
	nonMember := seedUser(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)

	teamID := seedTeam(t, db, ws, "attainment-team")
	for _, uid := range []string{member1, member2} {
		if _, err := db.Exec(
			`INSERT INTO team_membership (workspace_id, team_id, user_id) VALUES ($1,$2,$3)`,
			ws, teamID, uid,
		); err != nil {
			t.Fatalf("seed team_membership for %s: %v", uid, err)
		}
	}

	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	inPeriod := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	qID := seedQuota(t, store, ws, nil, &teamID, periodStart, periodEnd, 10000000, "EUR")
	// Both members' deals count; non-member's deal must not.
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, member1, 3000000, "EUR", "1.0000000000", inPeriod)
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, member2, 4000000, "EUR", "1.0000000000", inPeriod)
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, nonMember, 9999999, "EUR", "1.0000000000", inPeriod)

	att, err := store.Attainment(ctx, qID, ws)
	if err != nil {
		t.Fatalf("Attainment (team-scoped): %v", err)
	}
	if att.ClosedWonMinor != 7000000 {
		t.Errorf("team-scoped closed_won_minor = %d, want 7000000 (non-member excluded)", att.ClosedWonMinor)
	}
	if len(att.ContributingDeals) != 2 {
		t.Errorf("contributing_deals len = %d, want 2", len(att.ContributingDeals))
	}
}

func TestQuotaStore_Attainment_PeriodBoundary(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	ownerID := seedUser(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)

	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	dayAfterEnd := periodEnd.Add(24 * time.Hour)

	qID := seedQuota(t, store, ws, &ownerID, nil, periodStart, periodEnd, 10000000, "EUR")
	// Exactly on period_start: counts.
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, ownerID, 1000000, "EUR", "1.0000000000", periodStart)
	// Exactly on period_end: counts.
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, ownerID, 2000000, "EUR", "1.0000000000", periodEnd)
	// Day after period_end: must NOT count.
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, ownerID, 9999999, "EUR", "1.0000000000", dayAfterEnd)

	att, err := store.Attainment(ctx, qID, ws)
	if err != nil {
		t.Fatalf("Attainment (period boundary): %v", err)
	}
	if att.ClosedWonMinor != 3000000 {
		t.Errorf("period-boundary closed_won_minor = %d, want 3000000 (start+end count, day-after excluded)", att.ClosedWonMinor)
	}
	if len(att.ContributingDeals) != 2 {
		t.Errorf("contributing_deals len = %d, want 2", len(att.ContributingDeals))
	}
}

func TestQuotaStore_Attainment_CrossCurrency(t *testing.T) {
	// Quota in USD, workspace base_currency=EUR; seeded USD->EUR fx_rate at 0.9.
	// Also seeds a USD won deal to confirm amount_minor_base (already EUR-base) is used directly.
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db) // base_currency=EUR
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	today := time.Now().UTC()
	seedFXRate(t, db, ws, "USD", "EUR", "0.9000000000", today)

	ownerID := seedUser(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	inPeriod := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	// target_minor=10_000_000 USD; at rate 0.9 → round(10_000_000 * 0.9) = 9_000_000 EUR.
	qID := seedQuota(t, store, ws, &ownerID, nil, periodStart, periodEnd, 10000000, "USD")

	// USD won deal: fx_rate_to_base=0.9 → amount_minor_base = round(2_000_000 * 0.9) = 1_800_000 EUR.
	// The deal's amount_minor_base is already in base currency — no further conversion by Attainment.
	seedQuotaWonDeal(t, db, ws, pipelineID, stageID, ownerID, 2000000, "USD", "0.9000000000", inPeriod)

	att, err := store.Attainment(ctx, qID, ws)
	if err != nil {
		t.Fatalf("Attainment (cross-currency): %v", err)
	}
	if att.TargetMinor != 9000000 {
		t.Errorf("cross-currency target_minor = %d, want 9000000 (10M USD * 0.9)", att.TargetMinor)
	}
	if att.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR (workspace base)", att.Currency)
	}
	// closed_won_minor = amount_minor_base of the USD deal (already EUR, no further conversion).
	if att.ClosedWonMinor != 1800000 {
		t.Errorf("cross-currency closed_won_minor = %d, want 1800000 (2M USD deal @ 0.9)", att.ClosedWonMinor)
	}
}

func TestQuotaStore_Attainment_TargetZero(t *testing.T) {
	// target_minor==0 is refused before any deal or FX query.
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	ownerID := seedUser(t, db, ws)
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	// Use store.Create — target_minor=0 is valid in the DB (no CHECK constraint), only refused by Attainment.
	qID := seedQuota(t, store, ws, &ownerID, nil, periodStart, periodEnd, 0, "EUR")

	_, err := store.Attainment(ctx, qID, ws)
	if !errors.Is(err, records.ErrAttainmentTargetZero) {
		t.Errorf("target_minor=0: err = %v, want ErrAttainmentTargetZero", err)
	}
}

func TestQuotaStore_Attainment_MissingFXRate(t *testing.T) {
	// Cross-currency quota with no fx_rate row seeded → FXRateUnavailableError.
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db) // base_currency=EUR
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	ownerID := seedUser(t, db, ws)
	periodStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	// USD quota, no USD->EUR fx_rate seeded.
	qID := seedQuota(t, store, ws, &ownerID, nil, periodStart, periodEnd, 10000000, "USD")

	_, err := store.Attainment(ctx, qID, ws)
	var fxErr *deals.FXRateUnavailableError
	if !errors.As(err, &fxErr) {
		t.Errorf("missing FX rate: err = %v, want *deals.FXRateUnavailableError", err)
	}
}

func TestQuotaStore_Attainment_NotFound(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := pgtest.AppCtx(ws)
	store := records.NewQuotaStore(db)

	_, err := store.Attainment(ctx, "00000000-0000-0000-0000-000000000000", ws)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("nonexistent quota: err = %v, want errs.ErrNotFound", err)
	}
}

// Compile-time check: Attainment and AttainmentDeal are accessible via the records alias layer.
var (
	_ records.Attainment
	_ records.AttainmentDeal
)
