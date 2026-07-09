//go:build integration

// store_org_restore_test.go — ported from modules/directory/store_org_restore_test.go
// (package crmcore_test → package adapters_test; type refs updated to
// organizations/adapters and organizations/domain).
package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

const orgRestoreWS = "00000000-0000-0000-0000-000000000051"

func seedOrgRestoreWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		orgRestoreWS, "org-restore-ws", "org-restore-ws-"+time.Now().Format("20060102150405"),
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func orgRestoreCtx() context.Context {
	return crmctx.With(context.Background(), crmctx.Principal{TenantID: orgRestoreWS, UserID: "human:test"})
}

func TestOrgStore_Restore_ArchivedOrgWritesEventAndAudit(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	seedOrgRestoreWorkspace(t, db)

	store := orgAdapters.NewOrgStore(db)

	created, err := store.Create(orgRestoreCtx(), orgDomain.Organization{
		WorkspaceID: orgRestoreWS,
		DisplayName: "Restore Target",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	archived, err := store.Archive(orgRestoreCtx(), created.ID, orgRestoreWS)
	if err != nil {
		t.Fatalf("archive org: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("archive returned a live organization")
	}

	restored, err := store.Restore(orgRestoreCtx(), created.ID, orgRestoreWS)
	if err != nil {
		t.Fatalf("restore org: %v", err)
	}
	if restored.ID != created.ID {
		t.Fatalf("restore returned id=%s, want %s", restored.ID, created.ID)
	}
	if restored.ArchivedAt != nil {
		t.Fatal("restore returned an archived organization")
	}

	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox
		 WHERE workspace_id=$1::uuid AND entity_id=$2::uuid AND topic='organization.restored'`,
		orgRestoreWS, created.ID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count organization.restored events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("organization.restored event count = %d, want 1", eventCount)
	}

	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log
		 WHERE workspace_id=$1::uuid AND entity_type='organization' AND entity_id=$2::uuid AND action='restore'`,
		orgRestoreWS, created.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count organization restore audit rows: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("organization restore audit count = %d, want 1", auditCount)
	}
}

func TestOrgStore_Restore_RefusesLiveAndMergedRecords(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	seedOrgRestoreWorkspace(t, db)

	store := orgAdapters.NewOrgStore(db)

	live, err := store.Create(orgRestoreCtx(), orgDomain.Organization{
		WorkspaceID: orgRestoreWS,
		DisplayName: "Live Organization",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create live org: %v", err)
	}

	if _, err := store.Restore(orgRestoreCtx(), live.ID, orgRestoreWS); !errors.Is(err, errs.ErrNotArchived) {
		t.Fatalf("restore live org err = %v, want ErrNotArchived", err)
	}

	merged, err := store.Create(orgRestoreCtx(), orgDomain.Organization{
		WorkspaceID: orgRestoreWS,
		DisplayName: "Merged Organization",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create merged org: %v", err)
	}

	if _, err := db.ExecContext(context.Background(),
		`UPDATE organization
		 SET archived_at = now(), merged_into_id = $1::uuid
		 WHERE id = $2::uuid AND workspace_id = $3::uuid`,
		live.ID, merged.ID, orgRestoreWS); err != nil {
		t.Fatalf("seed merged organization state: %v", err)
	}

	if _, err := store.Restore(orgRestoreCtx(), merged.ID, orgRestoreWS); !errors.Is(err, errs.ErrMergedRecord) {
		t.Fatalf("restore merged org err = %v, want ErrMergedRecord", err)
	}
}
