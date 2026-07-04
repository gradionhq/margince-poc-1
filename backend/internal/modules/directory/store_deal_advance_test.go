//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const advanceTestWorkspaceID = "00000000-0000-0000-0000-000000000a12"

func setupAdvanceFixtures(t *testing.T, db *sql.DB, tag string) (pipelineID, openStageID, wonStageID, lostStageID string) {
	t.Helper()
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t12-adv-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, advanceTestWorkspaceID, "t12-adv-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, advanceTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		advanceTestWorkspaceID, "Pipeline "+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Open '||$3, 1, 'open', 20) RETURNING id`,
		advanceTestWorkspaceID, pipelineID, tag).Scan(&openStageID); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Won '||$3, 2, 'won', 100) RETURNING id`,
		advanceTestWorkspaceID, pipelineID, tag).Scan(&wonStageID); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Lost '||$3, 3, 'lost', 0) RETURNING id`,
		advanceTestWorkspaceID, pipelineID, tag).Scan(&lostStageID); err != nil {
		t.Fatalf("seed lost stage: %v", err)
	}
	return pipelineID, openStageID, wonStageID, lostStageID
}

func TestDealStore_Advance_OpenToWon_SingleWriteEachTable(t *testing.T) {
	db := openCreateTestDB(t)
	pipelineID, openA, wonA, _ := setupAdvanceFixtures(t, db, "o2w")
	store := crmcore.NewDealStore(db)
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1, 'EUR', 'EUR', 1, current_date) ON CONFLICT DO NOTHING`, advanceTestWorkspaceID); err != nil {
		t.Fatalf("seed fx_rate: %v", err)
	}

	d := crmcore.NewDeal("Deal o2w", pipelineID, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = advanceTestWorkspaceID
	amountMinor := int64(10000)
	currency := "EUR"
	d.AmountMinor = &amountMinor
	d.Currency = &currency
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := store.Advance(ctx, created.ID, advanceTestWorkspaceID, crmcore.AdvanceInput{
		ToStageID: wonA, Status: "won",
	}, 0, "human:test")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if updated.Status != "won" {
		t.Fatalf("expected status=won, got %q", updated.Status)
	}
	if updated.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}

	var historyCount int
	if err := db.QueryRow(`SELECT count(*) FROM deal_stage_history WHERE deal_id=$1 AND to_stage_id=$2`,
		created.ID, wonA).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if historyCount != 1 {
		t.Fatalf("expected exactly 1 advance history row, got %d", historyCount)
	}

	var auditCount int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_id=$1 AND action='advance_stage'`,
		created.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected exactly 1 audit row, got %d", auditCount)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE entity_id=$1 AND topic='deal.stage_changed'`,
		created.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("expected exactly 1 deal.stage_changed event, got %d", eventCount)
	}

	var payloadBytes []byte
	if err := db.QueryRow(`SELECT payload FROM event_outbox WHERE entity_id=$1 AND topic='deal.stage_changed'`,
		created.ID).Scan(&payloadBytes); err != nil {
		t.Fatal(err)
	}
	var got struct {
		AmountMinor *int64  `json:"amount_minor"`
		Currency    *string `json:"currency"`
	}
	if err := json.Unmarshal(payloadBytes, &got); err != nil {
		t.Fatalf("event payload amount_minor/currency must marshal as scalars, not sql.Null wrappers: %v (raw=%s)", err, payloadBytes)
	}
	if got.AmountMinor == nil || *got.AmountMinor != amountMinor {
		t.Fatalf("expected amount_minor=%d in event payload, got %v", amountMinor, got.AmountMinor)
	}
	if got.Currency == nil || *got.Currency != currency {
		t.Fatalf("expected currency=%s in event payload, got %v", currency, got.Currency)
	}
}

// basecurNoFXWorkspaceID is a dedicated workspace (distinct from
// advanceTestWorkspaceID) so this test's "zero fx_rate rows on file" premise
// can't be invalidated by other Advance tests inserting fx_rate rows into a
// shared workspace within the same test-DB run.
const basecurNoFXWorkspaceID = "00000000-0000-0000-0000-000000000a13"

func TestDealStore_Advance_OpenToWon_BaseCurrencyNoFXRateRow_DefaultsToOne(t *testing.T) {
	db := openCreateTestDB(t)
	tag := fmt.Sprintf("basecur-nofx-%d", time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t21-basecur-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, basecurNoFXWorkspaceID, "t21-basecur-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, basecurNoFXWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var pipelineID, openA, wonA string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		basecurNoFXWorkspaceID, "Pipeline "+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Open '||$3, 1, 'open', 20) RETURNING id`,
		basecurNoFXWorkspaceID, pipelineID, tag).Scan(&openA); err != nil {
		t.Fatalf("seed open stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'Won '||$3, 2, 'won', 100) RETURNING id`,
		basecurNoFXWorkspaceID, pipelineID, tag).Scan(&wonA); err != nil {
		t.Fatalf("seed won stage: %v", err)
	}

	// This workspace has never had an fx_rate row inserted (unlike the shared
	// advanceTestWorkspaceID used elsewhere in this file), so a deal
	// denominated in the workspace's own base currency must still close.
	var fxRateCount int
	if err := db.QueryRow(`SELECT count(*) FROM fx_rate WHERE workspace_id=$1`, basecurNoFXWorkspaceID).Scan(&fxRateCount); err != nil {
		t.Fatal(err)
	}
	if fxRateCount != 0 {
		t.Fatalf("test setup invariant: expected zero fx_rate rows, got %d", fxRateCount)
	}

	store := crmcore.NewDealStore(db)
	ctx := context.Background()

	d := crmcore.NewDeal("Deal basecur-nofx", pipelineID, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = basecurNoFXWorkspaceID
	amountMinor := int64(10000)
	currency := "EUR"
	d.AmountMinor = &amountMinor
	d.Currency = &currency
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := store.Advance(ctx, created.ID, basecurNoFXWorkspaceID, crmcore.AdvanceInput{
		ToStageID: wonA, Status: "won",
	}, 0, "human:test")
	if err != nil {
		t.Fatalf("Advance to won with no fx_rate row on file should succeed for a base-currency deal: %v", err)
	}
	if updated.FxRateToBase == nil || *updated.FxRateToBase != 1.0 {
		t.Fatalf("expected fx_rate_to_base=1.0 for base-currency close, got %v", updated.FxRateToBase)
	}
	if updated.FxRateDate == nil {
		t.Fatal("expected fx_rate_date to be set for base-currency close")
	}
}

func TestDealStore_Advance_StatusMismatchRejected(t *testing.T) {
	db := openCreateTestDB(t)
	pipelineID, openA, wonA, _ := setupAdvanceFixtures(t, db, "mismatch")
	store := crmcore.NewDealStore(db)
	ctx := context.Background()

	d := crmcore.NewDeal("Deal mismatch", pipelineID, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = advanceTestWorkspaceID
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = store.Advance(ctx, created.ID, advanceTestWorkspaceID, crmcore.AdvanceInput{
		ToStageID: wonA, Status: "open", // mismatches the target's "won" semantic
	}, 0, "human:test")
	if err == nil {
		t.Fatal("expected a status/semantic mismatch to be rejected")
	}
}

func TestDealStore_Advance_LostWithoutReasonRejected(t *testing.T) {
	db := openCreateTestDB(t)
	pipelineID, openA, _, lostA := setupAdvanceFixtures(t, db, "lostnoreason")
	store := crmcore.NewDealStore(db)
	ctx := context.Background()

	d := crmcore.NewDeal("Deal lost-no-reason", pipelineID, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = advanceTestWorkspaceID
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = store.Advance(ctx, created.ID, advanceTestWorkspaceID, crmcore.AdvanceInput{
		ToStageID: lostA, // no LostReason
	}, 0, "human:test")
	if err == nil {
		t.Fatal("expected advancing to a lost stage without lost_reason to be rejected")
	}
}

func TestDealStore_Advance_OpenToOpen_NoFXNoClosedAt(t *testing.T) {
	db := openCreateTestDB(t)
	pipe, err := seedTwoOpenStagePipeline(t, db, "o2o-real")
	if err != nil {
		t.Fatal(err)
	}
	store := crmcore.NewDealStore(db)
	ctx := context.Background()

	openA, openB := pipe.stageA, pipe.stageB
	d := crmcore.NewDeal("Deal o2o real", pipe.pipeline, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = advanceTestWorkspaceID
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := store.Advance(ctx, created.ID, advanceTestWorkspaceID, crmcore.AdvanceInput{
		ToStageID: openB,
	}, 0, "human:test")
	if err != nil {
		t.Fatalf("Advance open->open: %v", err)
	}
	if updated.Status != "open" || updated.ClosedAt != nil || updated.FxRateToBase != nil {
		t.Fatalf("open->open must not close or freeze FX: %+v", updated)
	}
}

func TestDealStore_Advance_Reopen_ClearsClosedAtLostReasonAndFX(t *testing.T) {
	db := openCreateTestDB(t)
	pipelineID, openA, _, lostA := setupAdvanceFixtures(t, db, "reopen")
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1, 'EUR', 'USD', 1.1, current_date) ON CONFLICT DO NOTHING`, advanceTestWorkspaceID); err != nil {
		t.Fatalf("seed fx_rate: %v", err)
	}
	store := crmcore.NewDealStore(db)
	ctx := context.Background()

	amount := int64(10000)
	currency := "EUR"
	d := crmcore.NewDeal("Deal reopen", pipelineID, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = advanceTestWorkspaceID
	d.AmountMinor = &amount
	d.Currency = &currency
	created, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	reason := "budget cut"
	closed, err := store.Advance(ctx, created.ID, advanceTestWorkspaceID, crmcore.AdvanceInput{
		ToStageID: lostA, LostReason: &reason,
	}, 0, "human:test")
	if err != nil {
		t.Fatalf("Advance to lost: %v", err)
	}
	if closed.ClosedAt == nil || closed.LostReason == nil || closed.FxRateToBase == nil {
		t.Fatalf("expected closed_at/lost_reason/fx_rate_to_base all set after close: %+v", closed)
	}

	reopened, err := store.Advance(ctx, created.ID, advanceTestWorkspaceID, crmcore.AdvanceInput{
		ToStageID: openA,
	}, 0, "human:test")
	if err != nil {
		t.Fatalf("Advance reopen: %v", err)
	}
	if reopened.ClosedAt != nil || reopened.LostReason != nil || reopened.FxRateToBase != nil || reopened.FxRateDate != nil {
		t.Fatalf("expected closed_at/lost_reason/fx fields all cleared on reopen: %+v", reopened)
	}
	if reopened.Status != "open" {
		t.Fatalf("expected status=open after reopen, got %q", reopened.Status)
	}

	var historyCount int
	if err := db.QueryRow(`SELECT count(*) FROM deal_stage_history WHERE deal_id=$1`, created.ID).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if historyCount != 3 { // create + close + reopen
		t.Fatalf("expected 3 history rows (create, close, reopen), got %d", historyCount)
	}
}

type twoOpenStagePipeline struct {
	pipeline, stageA, stageB string
}

func seedTwoOpenStagePipeline(t *testing.T, db *sql.DB, tag string) (twoOpenStagePipeline, error) {
	t.Helper()
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t12-2open-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, advanceTestWorkspaceID, "t12-2open-ws-"+tag); err != nil {
		return twoOpenStagePipeline{}, err
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, advanceTestWorkspaceID); err != nil {
		return twoOpenStagePipeline{}, err
	}
	var p twoOpenStagePipeline
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		advanceTestWorkspaceID, "2open "+tag).Scan(&p.pipeline); err != nil {
		return p, err
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'A '||$3, 1, 'open', 10) RETURNING id`,
		advanceTestWorkspaceID, p.pipeline, tag).Scan(&p.stageA); err != nil {
		return p, err
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
		VALUES (uuidv7(), $1, $2, 'B '||$3, 2, 'open', 40) RETURNING id`,
		advanceTestWorkspaceID, p.pipeline, tag).Scan(&p.stageB); err != nil {
		return p, err
	}
	return p, nil
}
