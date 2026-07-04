//go:build integration

package crmcore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/lib/pq"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func openPersonRestoreTestDB(t *testing.T) *sql.DB {
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

func personRestoreCtx(wsID string) context.Context {
	return crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
}

func TestPersonStore_Restore_ArchivedPersonWritesEventAndAudit(t *testing.T) {
	db := openPersonRestoreTestDB(t)
	const wsID = "00000000-0000-0000-0000-000000000031"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	store := NewPersonStore(db)
	ctx := personRestoreCtx(wsID)

	created, err := store.Create(ctx, Person{
		WorkspaceID: wsID,
		FullName:    "Restore Target",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}

	archived, err := store.Archive(ctx, created.ID, wsID)
	if err != nil {
		t.Fatalf("archive person: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("archive returned a live person")
	}

	restored, err := store.Restore(ctx, created.ID, wsID)
	if err != nil {
		t.Fatalf("restore person: %v", err)
	}
	if restored.ID != created.ID {
		t.Fatalf("restore returned id=%s, want %s", restored.ID, created.ID)
	}
	if restored.ArchivedAt != nil {
		t.Fatal("restore returned an archived person")
	}

	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox
		 WHERE workspace_id=$1::uuid AND entity_id=$2::uuid AND topic='person.restored'`,
		wsID, created.ID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count person.restored events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("person.restored event count = %d, want 1", eventCount)
	}

	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log
		 WHERE workspace_id=$1::uuid AND entity_type='person' AND entity_id=$2::uuid AND action='restore'`,
		wsID, created.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count person restore audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("person restore audit count = %d, want 1", auditCount)
	}
}

func TestPersonStore_Restore_RefusesLiveAndMergedRecords(t *testing.T) {
	db := openPersonRestoreTestDB(t)
	const wsID = "00000000-0000-0000-0000-000000000032"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	store := NewPersonStore(db)
	ctx := personRestoreCtx(wsID)

	live, err := store.Create(ctx, Person{
		WorkspaceID: wsID,
		FullName:    "Live Person",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create live person: %v", err)
	}

	if _, err := store.Restore(ctx, live.ID, wsID); !errors.Is(err, errs.ErrNotArchived) {
		t.Fatalf("restore live person err = %v, want ErrNotArchived", err)
	}

	merged, err := store.Create(ctx, Person{
		WorkspaceID: wsID,
		FullName:    "Merged Person",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create merged person: %v", err)
	}

	if _, err := db.ExecContext(context.Background(),
		`UPDATE person
		 SET archived_at = now(), merged_into_id = $1::uuid
		 WHERE id = $2::uuid AND workspace_id = $3::uuid`,
		live.ID, merged.ID, wsID); err != nil {
		t.Fatalf("seed merged person state: %v", err)
	}

	if _, err := store.Restore(ctx, merged.ID, wsID); !errors.Is(err, errs.ErrMergedRecord) {
		t.Fatalf("restore merged person err = %v, want ErrMergedRecord", err)
	}
}
