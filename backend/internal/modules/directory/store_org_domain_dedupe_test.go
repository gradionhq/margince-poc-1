//go:build integration

package crmcore

import (
	"context"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func TestOrgStoreCreateDomainDuplicateRejected(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := NewOrgStore(db)

	first := Organization{
		WorkspaceID: ws, DisplayName: "Acme", Source: "api", CapturedBy: "human:t",
		Domains: []OrganizationDomain{{Domain: "acme.com", IsPrimary: true}},
	}
	created, err := store.Create(ctx, first)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second := Organization{
		WorkspaceID: ws, DisplayName: "Acme Dup", Source: "api", CapturedBy: "human:t",
		Domains: []OrganizationDomain{{Domain: "ACME.com", IsPrimary: true}},
	}
	_, err = store.Create(ctx, second)
	var dup *ErrDuplicateDomain
	if !errors.As(err, &dup) {
		t.Fatalf("second create: want ErrDuplicateDomain, got %v", err)
	}
	if dup.ExistingID != created.ID {
		t.Fatalf("dup.ExistingID = %s, want %s", dup.ExistingID, created.ID)
	}
}
