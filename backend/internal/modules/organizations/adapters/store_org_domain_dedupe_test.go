//go:build integration

// store_org_domain_dedupe_test.go — ported from modules/directory/store_org_domain_dedupe_test.go
// (package crmcore → package adapters_test; type refs updated to organizations/adapters
// and organizations/domain).
package adapters_test

import (
	"context"
	"errors"
	"testing"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func TestOrgStoreCreateDomainDuplicateRejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := ids.New()
	pgtest.SeedWorkspace(t, db, ws)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})
	store := orgAdapters.NewOrgStore(db)

	first := orgDomain.Organization{
		WorkspaceID: ws, DisplayName: "Acme", Source: "api", CapturedBy: "human:t",
		Domains: []orgDomain.OrganizationDomain{{Domain: "acme.com", IsPrimary: true}},
	}
	created, err := store.Create(ctx, first, nil)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second := orgDomain.Organization{
		WorkspaceID: ws, DisplayName: "Acme Dup", Source: "api", CapturedBy: "human:t",
		Domains: []orgDomain.OrganizationDomain{{Domain: "ACME.com", IsPrimary: true}},
	}
	_, err = store.Create(ctx, second, nil)
	var dup *orgAdapters.ErrDuplicateDomain
	if !errors.As(err, &dup) {
		t.Fatalf("second create: want ErrDuplicateDomain, got %v", err)
	}
	if dup.ExistingID != created.ID {
		t.Fatalf("dup.ExistingID = %s, want %s", dup.ExistingID, created.ID)
	}
}
