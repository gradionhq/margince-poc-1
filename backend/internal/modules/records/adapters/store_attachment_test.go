//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/lib/pq" // registers the postgres driver for database/sql

	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// seedAttachmentWorkspace opens a workspace, sets RLS on the connection, and
// returns (ctx-with-principal, workspaceID). A bound person is seeded so the
// attachment has a real entity_id to point at.
func seedAttachmentWorkspace(t *testing.T, db *sql.DB) (context.Context, string, string) {
	t.Helper()
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:rd-t05-test", TenantID: ws})
	var personID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person (workspace_id, full_name, source, captured_by) VALUES ($1,'Att Person','test','human:rd-t05-test') RETURNING id`,
		ws).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return ctx, ws, personID
}

func newAttachmentFixture(ws, entityID string) domain.Attachment {
	p := prov.Provenance{Source: "test", CapturedBy: "human:rd-t05-test"}
	a := domain.NewAttachment(domain.EntityTypePerson, entityID, "doc.pdf", "application/pdf", 2048,
		"attachments/"+ws+"/"+entityID+"/doc.pdf", p)
	a.WorkspaceID = ws
	return a
}

func TestAttachmentStore_CreateGetArchive_RoundTrip(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	created, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ScanStatus != domain.ScanStatusScanning {
		t.Fatalf("expected fresh row scan_status=scanning, got %q", created.ScanStatus)
	}
	if created.ID == "" {
		t.Fatal("expected minted id")
	}

	got, err := s.Get(ctx, created.ID, ws)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.Filename != "doc.pdf" || got.ByteSize != 2048 {
		t.Fatalf("get returned mismatched row: %+v", got)
	}
	if got.StorageKey != created.StorageKey {
		t.Fatalf("storage_key round-trip mismatch: %q vs %q", got.StorageKey, created.StorageKey)
	}

	archived, err := s.Archive(ctx, created.ID, ws)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected archived_at set")
	}

	if _, err := s.Get(ctx, created.ID, ws); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for archived row's default Get, got %v", err)
	}
}

// Regression test for a live-stack UAT bug: GetAny must return an archived
// row that the archived_at-filtered Get 404s on — the single-item GET
// transport handler relies on this to keep archived attachments retrievable
// (disclosed-locked 200) instead of 404ing.
func TestAttachmentStore_GetAny_ReturnsArchivedRowThatGet404s(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	created, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Archive(ctx, created.ID, ws); err != nil {
		t.Fatalf("archive: %v", err)
	}

	if _, err := s.Get(ctx, created.ID, ws); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for archived row's filtered Get, got %v", err)
	}

	got, err := s.GetAny(ctx, created.ID, ws)
	if err != nil {
		t.Fatalf("GetAny: unexpected error for archived row: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetAny returned wrong row: got id %q, want %q", got.ID, created.ID)
	}
	if got.ArchivedAt == nil {
		t.Fatal("GetAny: expected archived_at set on the returned row")
	}
}

func TestAttachmentStore_Create_MissingProvenance_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	a := newAttachmentFixture(ws, personID)
	a.Source = ""
	a.CapturedBy = ""
	if _, err := s.Create(ctx, a); !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("expected ErrNullProvenance, got %v", err)
	}
}

func TestAttachmentStore_Create_WritesExactlyOneAuditLogRow(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	created, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE entity_type='attachment' AND entity_id=$1 AND action='create'`,
		created.ID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 create audit_log row, got %d", n)
	}
}

func TestAttachmentStore_Archive_WritesExactlyOneAuditLogRow(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	created, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Archive(ctx, created.ID, ws); err != nil {
		t.Fatalf("archive: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE entity_type='attachment' AND entity_id=$1 AND action='archive'`,
		created.ID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 archive audit_log row, got %d", n)
	}
}

func TestAttachmentStore_List_FiltersByEntityTypeAndID(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	// Second person to bind an unrelated attachment.
	var otherPersonID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO person (workspace_id, full_name, source, captured_by) VALUES ($1,'Other','test','human:rd-t05-test') RETURNING id`,
		ws).Scan(&otherPersonID); err != nil {
		t.Fatalf("seed other person: %v", err)
	}

	want, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create want: %v", err)
	}
	if _, err := s.Create(ctx, newAttachmentFixture(ws, otherPersonID)); err != nil {
		t.Fatalf("create other: %v", err)
	}

	items, _, err := s.List(ctx, ws, domain.EntityTypePerson, personID, "", 20, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != want.ID {
		t.Fatalf("expected exactly the personID-bound attachment, got %+v", items)
	}
}

func TestAttachmentStore_List_EmptyReturnsEmptyPage(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, _ := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	items, next, err := s.List(ctx, ws, "", "", "", 20, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("expected empty (non-nil) slice, got %+v", items)
	}
	if next != "" {
		t.Fatalf("expected no next cursor, got %q", next)
	}
}

func TestAttachmentStore_MarkScanResult_TransitionsToCleanOrBlocked(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	created, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ScanStatus != domain.ScanStatusScanning {
		t.Fatalf("fresh row should stay scanning until a verdict, got %q", created.ScanStatus)
	}

	marked, err := s.MarkScanResult(ctx, created.ID, ws, adapters.NewFakeScanner(domain.ScanStatusClean))
	if err != nil {
		t.Fatalf("mark scan result: %v", err)
	}
	if marked.ScanStatus != domain.ScanStatusClean {
		t.Fatalf("expected scan_status=clean after verdict, got %q", marked.ScanStatus)
	}
}

func TestAttachmentStore_MarkScanResult_RejectsInvalidStatus(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ctx, ws, personID := seedAttachmentWorkspace(t, db)
	s := adapters.NewAttachmentStore(db)

	created, err := s.Create(ctx, newAttachmentFixture(ws, personID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A scanner returning "scanning" (the row's own default) or anything not in
	// {clean,blocked} must be rejected, leaving the row unchanged.
	if _, err := s.MarkScanResult(ctx, created.ID, ws, adapters.NewFakeScanner(domain.ScanStatusScanning)); !errors.Is(err, adapters.ErrInvalidScanStatus) {
		t.Fatalf("expected ErrInvalidScanStatus for 'scanning' verdict, got %v", err)
	}
	if _, err := s.MarkScanResult(ctx, created.ID, ws, adapters.NewFakeScanner("bogus")); !errors.Is(err, adapters.ErrInvalidScanStatus) {
		t.Fatalf("expected ErrInvalidScanStatus for arbitrary verdict, got %v", err)
	}

	got, err := s.Get(ctx, created.ID, ws)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ScanStatus != domain.ScanStatusScanning {
		t.Fatalf("row must stay 'scanning' after a rejected verdict, got %q", got.ScanStatus)
	}
}
