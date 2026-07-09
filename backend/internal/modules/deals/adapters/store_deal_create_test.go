//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func openCreateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return pgtest.OpenTestDB(t)
}

const createTestWorkspaceID = "00000000-0000-0000-0000-000000000002"

func setupCreateFixtures(t *testing.T, db *sql.DB, tag string) (pipelineID, stageID, otherPipelineStageID string) {
	t.Helper()
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t11-create-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, createTestWorkspaceID, "t11-create-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, createTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var pipelineA, pipelineB string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name)
		VALUES (uuidv7(), $1, $2) RETURNING id`, createTestWorkspaceID, fmt.Sprintf("Pipeline A %s", tag)).Scan(&pipelineA); err != nil {
		t.Fatalf("seed pipeline A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name)
		VALUES (uuidv7(), $1, $2) RETURNING id`, createTestWorkspaceID, fmt.Sprintf("Pipeline B %s", tag)).Scan(&pipelineB); err != nil {
		t.Fatalf("seed pipeline B: %v", err)
	}
	var stageA, stageB string
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position)
		VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`, createTestWorkspaceID, pipelineA, fmt.Sprintf("Qualify A %s", tag)).Scan(&stageA); err != nil {
		t.Fatalf("seed stage A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position)
		VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`, createTestWorkspaceID, pipelineB, fmt.Sprintf("Qualify B %s", tag)).Scan(&stageB); err != nil {
		t.Fatalf("seed stage B: %v", err)
	}
	return pipelineA, stageA, stageB
}

func TestDealStore_Create_WritesHistoryRowAndStageCheck(t *testing.T) {
	db := openCreateTestDB(t)
	pipelineID, stageID, otherPipelineStageID := setupCreateFixtures(t, db, "history")
	store := adapters.NewDealStore(db)
	ctx := context.Background()

	d := domain.NewDeal("Acme deal", pipelineID, stageID,
		prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = createTestWorkspaceID
	created, err := store.Create(ctx, d, "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	var fromStageID sql.NullString
	var toStageID string
	if err := db.QueryRow(`SELECT from_stage_id, to_stage_id FROM deal_stage_history
		WHERE deal_id=$1`, created.ID).Scan(&fromStageID, &toStageID); err != nil {
		t.Fatalf("expected a deal_stage_history row: %v", err)
	}
	if fromStageID.Valid {
		t.Fatalf("expected from_stage_id NULL on create, got %v", fromStageID.String)
	}
	if toStageID != stageID {
		t.Fatalf("to_stage_id = %s, want %s", toStageID, stageID)
	}
	if created.StageEnteredAt == nil {
		t.Fatal("expected StageEnteredAt to be set from the history row")
	}

	var eventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='deal.created' AND entity_id=$1`,
		created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("query event_outbox: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected exactly 1 deal.created event, got %d", eventCount)
	}

	bad := domain.NewDeal("Bad deal", pipelineID, otherPipelineStageID,
		prov.Provenance{Source: "test", CapturedBy: "human:test"})
	bad.WorkspaceID = createTestWorkspaceID
	_, err = store.Create(ctx, bad, "", nil)
	if err == nil {
		t.Fatal("expected ErrStageNotInPipeline, got nil")
	}
	if !errors.Is(err, errs.ErrStageNotInPipeline) {
		t.Fatalf("expected ErrStageNotInPipeline, got %v", err)
	}
}

func TestDealStore_Create_IdempotencyKeyReplay(t *testing.T) {
	db := openCreateTestDB(t)
	pipelineID, stageID, _ := setupCreateFixtures(t, db, fmt.Sprintf("replay-%d", time.Now().UnixNano()))
	store := adapters.NewDealStore(db)
	ctx := context.Background()
	dealName := fmt.Sprintf("Replay deal %d", time.Now().UnixNano())

	d := domain.NewDeal(dealName, pipelineID, stageID,
		prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = createTestWorkspaceID
	first, err := store.Create(ctx, d, "idem-key-1", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, ok, err := store.FindByIdempotencyKey(ctx, createTestWorkspaceID, "idem-key-1")
	if err != nil {
		t.Fatalf("FindByIdempotencyKey: %v", err)
	}
	if !ok {
		t.Fatal("expected FindByIdempotencyKey to find the deal")
	}
	if found.ID != first.ID {
		t.Fatalf("found.ID = %s, want %s", found.ID, first.ID)
	}

	var dealCount int
	if err := db.QueryRow(`SELECT count(*) FROM deal WHERE workspace_id=$1 AND name=$2`,
		createTestWorkspaceID, dealName).Scan(&dealCount); err != nil {
		t.Fatalf("count deals: %v", err)
	}
	if dealCount != 1 {
		t.Fatalf("expected exactly 1 deal row, got %d (idempotency replay must not duplicate)", dealCount)
	}
}
