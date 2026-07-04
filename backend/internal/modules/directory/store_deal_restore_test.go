//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const dealRestoreWS = "00000000-0000-0000-0000-000000000061"

func openDealRestoreTestDB(t *testing.T) *sql.DB {
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

func seedDealRestoreFixtures(t *testing.T, db *sql.DB) (pipelineID, stageID string) {
	t.Helper()
	tag := time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, dealRestoreWS); err != nil {
		t.Fatal("set rls guc:", err)
	}
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		dealRestoreWS, "deal-restore-ws", "deal-restore-ws-"+tag,
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO pipeline (workspace_id, name) VALUES ($1::uuid, $2) RETURNING id`,
		dealRestoreWS, "Deal Restore Pipeline "+tag,
	).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic)
		 VALUES ($1::uuid, $2::uuid, $3, 0, 'open') RETURNING id`,
		dealRestoreWS, pipelineID, "Open "+tag,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	return pipelineID, stageID
}

func TestDealStore_Restore_HappyPath(t *testing.T) {
	db := openDealRestoreTestDB(t)
	pipelineID, stageID := seedDealRestoreFixtures(t, db)
	ctx := context.Background()
	store := crmcore.NewDealStore(db)

	d := crmcore.NewDeal("Restorable Deal", pipelineID, stageID, prov.Provenance{
		Source: "test", CapturedBy: "human:test",
	})
	d.WorkspaceID = dealRestoreWS
	d, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.Archive(ctx, d.ID, dealRestoreWS); err != nil {
		t.Fatalf("archive: %v", err)
	}

	restored, err := store.Restore(ctx, d.ID, dealRestoreWS)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatalf("want archived_at nil after restore, got %v", restored.ArchivedAt)
	}

	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox WHERE topic='deal.restored' AND entity_id=$1::uuid`,
		d.ID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count event_outbox: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("want 1 deal.restored outbox row, got %d", eventCount)
	}
	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='deal' AND entity_id=$1::uuid AND action='restore'`,
		d.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("want 1 audit_log restore row, got %d", auditCount)
	}
}

func TestDealStore_Restore_LiveRecordRefused(t *testing.T) {
	db := openDealRestoreTestDB(t)
	pipelineID, stageID := seedDealRestoreFixtures(t, db)
	ctx := context.Background()
	store := crmcore.NewDealStore(db)

	d := crmcore.NewDeal("Already Live Deal", pipelineID, stageID, prov.Provenance{
		Source: "test", CapturedBy: "human:test",
	})
	d.WorkspaceID = dealRestoreWS
	d, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := store.Restore(ctx, d.ID, dealRestoreWS); !errors.Is(err, errs.ErrNotArchived) {
		t.Fatalf("want errs.ErrNotArchived restoring a live deal, got %v", err)
	}
}
