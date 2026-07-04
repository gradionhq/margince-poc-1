//go:build integration

package crmcore

import (
	"context"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func TestPersonStoreCreateEmailDuplicateRejected(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := NewPersonStore(db)

	first := NewPerson("Alice", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	first.WorkspaceID = ws
	created, err := store.Create(ctx, first, []PersonEmailInput{{Email: "alice@acme.com", IsPrimary: true}})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	second := NewPerson("Alice Duplicate", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	second.WorkspaceID = ws
	_, err = store.Create(ctx, second, []PersonEmailInput{{Email: "ALICE@acme.com", IsPrimary: true}})
	var dup *ErrDuplicateEmail
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
	store := NewPersonStore(db)

	p := NewPerson("Bob", prov.Provenance{Source: "api", CapturedBy: "human:t"})
	p.WorkspaceID = ws
	created, err := store.Create(ctx, p, []PersonEmailInput{{Email: "bob@acme.com", IsPrimary: true}})
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
