//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
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

	d := crmcore.NewDeal("Deal o2w", pipelineID, openA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = advanceTestWorkspaceID
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
