//go:build integration

package records_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// seedPipelineStage creates a throwaway pipeline + single "open"-semantic stage in ws and
// returns the stage id (deal.stage_id/pipeline_id are NOT NULL FKs) -- mirrors
// backend/internal/modules/organizations/adapters/helpers_shared_test.go's mkDealForMergeTest.
func seedPipelineStage(t *testing.T, db *sql.DB, ws string) (pipelineID, stageID string) {
	t.Helper()
	if err := db.QueryRow(
		`INSERT INTO pipeline (workspace_id, name, is_default) VALUES ($1,'RDT08Pipeline',true) RETURNING id`,
		ws,
	).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic) VALUES ($1,$2,'Open',1,'open') RETURNING id`,
		ws, pipelineID,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	return pipelineID, stageID
}

// seedOpenDeal inserts an 'open'-status deal with the given (nullable) amountMinor/fxRate and
// organization, returning its id.
func seedOpenDeal(t *testing.T, db *sql.DB, ws, pipelineID, stageID string, orgID *string, amountMinor *int64, fxRate *string) string {
	t.Helper()
	var currency *string
	if amountMinor != nil {
		c := "USD"
		currency = &c
	}
	var dealID string
	err := db.QueryRow(
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, organization_id, amount_minor,
		                    currency, fx_rate_to_base, status, source, captured_by)
		 VALUES ($1,'RDT08Deal',$2,$3,$4,$5,$6,$7,'open','api','human:t')
		 RETURNING id`,
		ws, pipelineID, stageID, orgID, amountMinor, currency, fxRate,
	).Scan(&dealID)
	if err != nil {
		t.Fatalf("seed open deal: %v", err)
	}
	return dealID
}

// TestDealAmountMinorBase_Values proves the same-row GENERATED column's actual computed values:
// both inputs present -> the expected base-currency amount; either input missing -> NULL, the
// honest "not computable yet" state (never an invented default).
func TestDealAmountMinorBase_Values(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)

	amount := int64(100000)
	fx := "1.1000000000"
	presentDealID := seedOpenDeal(t, db, ws, pipelineID, stageID, nil, &amount, &fx)

	var gotPresent sql.NullInt64
	if err := db.QueryRow(`SELECT amount_minor_base FROM deal WHERE id = $1`, presentDealID).Scan(&gotPresent); err != nil {
		t.Fatal(err)
	}
	if !gotPresent.Valid || gotPresent.Int64 != 110000 {
		t.Errorf("amount_minor_base with both inputs present = %+v, want 110000", gotPresent)
	}

	missingFxDealID := seedOpenDeal(t, db, ws, pipelineID, stageID, nil, &amount, nil)
	var gotMissingFx sql.NullInt64
	if err := db.QueryRow(`SELECT amount_minor_base FROM deal WHERE id = $1`, missingFxDealID).Scan(&gotMissingFx); err != nil {
		t.Fatal(err)
	}
	if gotMissingFx.Valid {
		t.Errorf("amount_minor_base with fx_rate_to_base NULL = %+v, want NULL (not computable yet)", gotMissingFx)
	}

	missingAmountDealID := seedOpenDeal(t, db, ws, pipelineID, stageID, nil, nil, &fx)
	var gotMissingAmount sql.NullInt64
	if err := db.QueryRow(`SELECT amount_minor_base FROM deal WHERE id = $1`, missingAmountDealID).Scan(&gotMissingAmount); err != nil {
		t.Fatal(err)
	}
	if gotMissingAmount.Valid {
		t.Errorf("amount_minor_base with amount_minor NULL = %+v, want NULL (not computable yet)", gotMissingAmount)
	}
}

// TestOrgOpenPipelineRollup_Bound is the bound test proper: proves the cross-record aggregate is
// served under the database-computed, no-interpreter bound with an honest "not computable yet"
// state for missing inputs, distinct from a genuine empty state.
func TestOrgOpenPipelineRollup_Bound(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	pipelineID, stageID := seedPipelineStage(t, db, ws)
	ctx := context.Background()

	mkOrg := func(name string) string {
		var id string
		if err := db.QueryRowContext(
			ctx,
			`INSERT INTO organization (workspace_id, name, source, captured_by) VALUES ($1,$2,'api','human:t') RETURNING id`,
			ws, name,
		).Scan(&id); err != nil {
			t.Fatalf("seed organization %s: %v", name, err)
		}
		return id
	}

	// Org A: one open deal, both inputs present -> correct aggregate.
	orgA := mkOrg("RDT08 Org A")
	amountA := int64(200000)
	fxA := "1.0000000000"
	seedOpenDeal(t, db, ws, pipelineID, stageID, &orgA, &amountA, &fxA)

	var gotSum sql.NullInt64
	var gotCount int
	if err := db.QueryRowContext(
		ctx,
		`SELECT open_pipeline_minor_base, open_deal_count FROM organization_open_pipeline_rollup WHERE organization_id = $1`,
		orgA,
	).Scan(&gotSum, &gotCount); err != nil {
		t.Fatalf("org A rollup: %v", err)
	}
	if !gotSum.Valid || gotSum.Int64 != 200000 || gotCount != 1 {
		t.Errorf("org A rollup = (sum=%+v, count=%d), want (sum=200000, count=1)", gotSum, gotCount)
	}

	// Org B: no deals at all -> no row (never a fabricated zero).
	orgB := mkOrg("RDT08 Org B")
	err := db.QueryRowContext(
		ctx,
		`SELECT open_pipeline_minor_base FROM organization_open_pipeline_rollup WHERE organization_id = $1`,
		orgB,
	).Scan(&gotSum)
	if err != sql.ErrNoRows {
		t.Errorf("org B (no deals) rollup: want sql.ErrNoRows, got err=%v", err)
	}

	// Org C: one open deal, missing FX input -> a row exists (contributing deal is real), but the
	// aggregate is NULL -- "not computable yet", distinct from org B's genuine no-row empty state.
	orgC := mkOrg("RDT08 Org C")
	amountC := int64(50000)
	seedOpenDeal(t, db, ws, pipelineID, stageID, &orgC, &amountC, nil)
	if err := db.QueryRowContext(
		ctx,
		`SELECT open_pipeline_minor_base, open_deal_count FROM organization_open_pipeline_rollup WHERE organization_id = $1`,
		orgC,
	).Scan(&gotSum, &gotCount); err != nil {
		t.Fatalf("org C rollup: %v", err)
	}
	if gotSum.Valid {
		t.Errorf("org C rollup sum = %+v, want NULL (not computable yet, deal present but FX missing)", gotSum)
	}
	if gotCount != 1 {
		t.Errorf("org C rollup count = %d, want 1 (the contributing deal still counts, only its amount is uncomputable)", gotCount)
	}
}
