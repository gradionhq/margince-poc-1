//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	adapters "github.com/gradionhq/margince/backend/internal/modules/people/adapters"
	domain "github.com/gradionhq/margince/backend/internal/modules/people/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func TestPersonStoreCreateEmailDuplicateRejected(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := adapters.NewPersonStore(db)

	first := domain.NewPerson("Alice", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	first.WorkspaceID = ws
	created, err := store.Create(ctx, first, []domain.PersonEmailInput{{Email: "alice@acme.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	second := domain.NewPerson("Alice Duplicate", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	second.WorkspaceID = ws
	_, err = store.Create(ctx, second, []domain.PersonEmailInput{{Email: "ALICE@acme.com", IsPrimary: true}})
	var dup *adapters.ErrDuplicateEmail
	if !errors.As(err, &dup) {
		t.Fatalf("second create: want ErrDuplicateEmail, got %v", err)
	}
	if dup.ExistingID != created.ID {
		t.Fatalf("dup.ExistingID = %s, want %s", dup.ExistingID, created.ID)
	}
}

func TestPersonStoreCreateEmailStoresRow(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := adapters.NewPersonStore(db)

	p := domain.NewPerson("Bob", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	p.WorkspaceID = ws
	created, err := store.Create(ctx, p, []domain.PersonEmailInput{{Email: "bob@acme.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var email string
	var isPrimary bool
	if err := db.QueryRow(`SELECT email, is_primary FROM person_email WHERE person_id=$1::uuid`, created.ID).
		Scan(&email, &isPrimary); err != nil {
		t.Fatalf("select person_email: %v", err)
	}
	if email != "bob@acme.com" || !isPrimary {
		t.Fatalf("got email=%q is_primary=%v, want bob@acme.com/true", email, isPrimary)
	}
}

// TestPersonStoreArchiveCascadesEmailArchivedAt proves that archiving a
// person cascades archived_at onto their person_email rows (T23 UAT
// follow-up), so a brand-new person can reuse the archived person's email
// without a false 409 (PO-AC-16/PO-AC-6 live-scoped dedupe).
func TestPersonStoreArchiveCascadesEmailArchivedAt(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := adapters.NewPersonStore(db)

	a := domain.NewPerson("A", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	a.WorkspaceID = ws
	createdA, err := store.Create(ctx, a, []domain.PersonEmailInput{{Email: "x@example.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}

	if _, err := store.Archive(ctx, createdA.ID, ws); err != nil {
		t.Fatalf("archive A: %v", err)
	}

	var archivedAt sql.NullTime
	if err := db.QueryRow(`SELECT archived_at FROM person_email WHERE person_id=$1::uuid`, createdA.ID).
		Scan(&archivedAt); err != nil {
		t.Fatalf("select person_email for A: %v", err)
	}
	if !archivedAt.Valid {
		t.Fatal("person_email.archived_at for archived person A is still NULL, want set")
	}

	b := domain.NewPerson("B", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	b.WorkspaceID = ws
	createdB, err := store.Create(ctx, b, []domain.PersonEmailInput{{Email: "x@example.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("create B with A's archived email: want success, got %v", err)
	}
	if createdB.ID == "" {
		t.Fatal("create B: want a non-empty id")
	}
}

// TestPersonStoreRestoreCascadesEmailArchivedAtAndDetectsCollision proves the
// inverse cascade (Restore clears person_email.archived_at) and that the
// restore-time duplicate-email check now actually observes a real collision
// against a live person created while the original owner was archived (T23
// UAT follow-up).
func TestPersonStoreRestoreCascadesEmailArchivedAtAndDetectsCollision(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := adapters.NewPersonStore(db)

	a := domain.NewPerson("A", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	a.WorkspaceID = ws
	createdA, err := store.Create(ctx, a, []domain.PersonEmailInput{{Email: "x@example.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := store.Archive(ctx, createdA.ID, ws); err != nil {
		t.Fatalf("archive A: %v", err)
	}

	c := domain.NewPerson("C", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	c.WorkspaceID = ws
	createdC, err := store.Create(ctx, c, []domain.PersonEmailInput{{Email: "x@example.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("create C with A's archived email: %v", err)
	}

	_, err = store.Restore(ctx, createdA.ID, ws)
	var dup *adapters.ErrDuplicateEmail
	if !errors.As(err, &dup) {
		t.Fatalf("restore A: want ErrDuplicateEmail citing C, got %v", err)
	}
	if dup.ExistingID != createdC.ID {
		t.Fatalf("dup.ExistingID = %s, want %s (C)", dup.ExistingID, createdC.ID)
	}
}

// TestPersonStoreRestoreClearsEmailArchivedAt proves restoring a person
// (with no live collision) clears person_email.archived_at back to NULL for
// that person's own email rows.
func TestPersonStoreRestoreClearsEmailArchivedAt(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := adapters.NewPersonStore(db)

	a := domain.NewPerson("A", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	a.WorkspaceID = ws
	createdA, err := store.Create(ctx, a, []domain.PersonEmailInput{{Email: "restore-clear@example.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := store.Archive(ctx, createdA.ID, ws); err != nil {
		t.Fatalf("archive A: %v", err)
	}

	if _, err := store.Restore(ctx, createdA.ID, ws); err != nil {
		t.Fatalf("restore A: %v", err)
	}

	var archivedAt sql.NullTime
	if err := db.QueryRow(`SELECT archived_at FROM person_email WHERE person_id=$1::uuid`, createdA.ID).
		Scan(&archivedAt); err != nil {
		t.Fatalf("select person_email for A: %v", err)
	}
	if archivedAt.Valid {
		t.Fatal("person_email.archived_at for restored person A is still set, want NULL")
	}
}
