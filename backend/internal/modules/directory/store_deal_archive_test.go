//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	// Registers the postgres driver so sql.Open("postgres", ...) in openDealArchiveTestDB resolves; only the driver's side-effecting init() is used.
	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const dealArchiveWS = "00000000-0000-0000-0000-000000000062"

func openDealArchiveTestDB(t *testing.T) *sql.DB {
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

func seedDealArchiveFixtures(t *testing.T, db *sql.DB) (pipelineID, stageID string) {
	t.Helper()
	tag := time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, dealArchiveWS); err != nil {
		t.Fatal("set rls guc:", err)
	}
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		dealArchiveWS, "deal-archive-ws", "deal-archive-ws-"+tag,
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO pipeline (workspace_id, name) VALUES ($1::uuid, $2) RETURNING id`,
		dealArchiveWS, "Deal Archive Pipeline "+tag,
	).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic)
		 VALUES ($1::uuid, $2::uuid, $3, 0, 'open') RETURNING id`,
		dealArchiveWS, pipelineID, "Open "+tag,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	return pipelineID, stageID
}

func createArchivableDeal(ctx context.Context, t *testing.T, store *crmcore.DealStore, pipelineID, stageID, name string) crmcore.Deal {
	t.Helper()
	d := crmcore.NewDeal(name, pipelineID, stageID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = dealArchiveWS
	d, err := store.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return d
}

func assertDealArchiveEventAndAudit(t *testing.T, db *sql.DB, dealID string, wantCount int) {
	t.Helper()
	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox WHERE topic='deal.archived' AND entity_id=$1::uuid`,
		dealID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count event_outbox: %v", err)
	}
	if eventCount != wantCount {
		t.Fatalf("want %d deal.archived outbox row(s), got %d", wantCount, eventCount)
	}
	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='deal' AND entity_id=$1::uuid AND action='archive'`,
		dealID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount != wantCount {
		t.Fatalf("want %d audit_log archive row(s), got %d", wantCount, auditCount)
	}
}

func TestDealStore_Archive_WritesEventAndAudit(t *testing.T) {
	db := openDealArchiveTestDB(t)
	pipelineID, stageID := seedDealArchiveFixtures(t, db)
	ctx := context.Background()
	store := crmcore.NewDealStore(db)

	d := createArchivableDeal(ctx, t, store, pipelineID, stageID, "Archivable Deal")

	archived, err := store.Archive(ctx, d.ID, dealArchiveWS)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatalf("want archived_at set after archive, got nil")
	}

	assertDealArchiveEventAndAudit(t, db, d.ID, 1)
}

func TestDealStore_Archive_NonExistentReturnsNotFound(t *testing.T) {
	db := openDealArchiveTestDB(t)
	_, _ = seedDealArchiveFixtures(t, db)
	ctx := context.Background()
	store := crmcore.NewDealStore(db)

	_, err := store.Archive(ctx, "00000000-0000-0000-0000-000000000099", dealArchiveWS)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("want errs.ErrNotFound for nonexistent deal, got %v", err)
	}
}

func TestDealStore_Archive_AlreadyArchivedIsIdempotent(t *testing.T) {
	db := openDealArchiveTestDB(t)
	pipelineID, stageID := seedDealArchiveFixtures(t, db)
	ctx := context.Background()
	store := crmcore.NewDealStore(db)

	d := createArchivableDeal(ctx, t, store, pipelineID, stageID, "Idempotent Deal")

	// Archive once
	_, err := store.Archive(ctx, d.ID, dealArchiveWS)
	if err != nil {
		t.Fatalf("first archive: %v", err)
	}

	// Archive again — should be idempotent, no error
	_, err = store.Archive(ctx, d.ID, dealArchiveWS)
	if err != nil {
		t.Fatalf("second archive (idempotent): %v", err)
	}

	// Verify only one event and one audit row were written (not two)
	assertDealArchiveEventAndAudit(t, db, d.ID, 1)
}
