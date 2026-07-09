//go:build integration

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

const orgUpdateWS = "00000000-0000-0000-0000-000000000052"

func seedOrgUpdateWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		orgUpdateWS, "org-update-ws", "org-update-ws-"+time.Now().Format("20060102150405"),
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func orgUpdateCtx() context.Context {
	return crmctx.With(context.Background(), crmctx.Principal{TenantID: orgUpdateWS, UserID: "human:test"})
}

func TestOrgStore_Update_SetsParentOrgID(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	seedOrgUpdateWorkspace(t, db)

	store := orgAdapters.NewOrgStore(db)
	ctx := orgUpdateCtx()

	parent, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "Parent Corp",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create parent org: %v", err)
	}

	child, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "Child Corp",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create child org: %v", err)
	}

	updated, err := store.Update(ctx, child.ID, orgUpdateWS, map[string]any{"parent_org_id": parent.ID}, 0)
	if err != nil {
		t.Fatalf("update child parent_org_id: %v", err)
	}
	if updated.ParentOrgID == nil || *updated.ParentOrgID != parent.ID {
		t.Fatalf("Update returned ParentOrgID=%v, want %s", updated.ParentOrgID, parent.ID)
	}

	// Re-fetch independently to prove the write actually persisted (not just an in-memory echo).
	fetched, err := store.Get(ctx, child.ID, orgUpdateWS)
	if err != nil {
		t.Fatalf("re-fetch child: %v", err)
	}
	if fetched.ParentOrgID == nil || *fetched.ParentOrgID != parent.ID {
		t.Fatalf("Get after update returned ParentOrgID=%v, want %s", fetched.ParentOrgID, parent.ID)
	}
}

func TestOrgStore_Update_RejectsCyclicParent(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	seedOrgUpdateWorkspace(t, db)

	store := orgAdapters.NewOrgStore(db)
	ctx := orgUpdateCtx()

	a, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "Org A",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create org a: %v", err)
	}

	b, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "Org B",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create org b: %v", err)
	}

	// Seed b -> a edge directly (Create doesn't accept parent_org_id).
	if _, err := db.Exec(
		`UPDATE organization SET parent_org_id=$1::uuid WHERE id=$2::uuid AND workspace_id=$3::uuid`,
		a.ID, b.ID, orgUpdateWS,
	); err != nil {
		t.Fatalf("seed b->a parent edge: %v", err)
	}

	// Attempt a -> b would make a its own descendant's child (cycle).
	_, cycleErr := store.Update(ctx, a.ID, orgUpdateWS, map[string]any{"parent_org_id": b.ID}, 0)
	if !errors.Is(cycleErr, errs.ErrOrganizationCycle) {
		t.Fatalf("Update cycle err = %v, want errs.ErrOrganizationCycle", cycleErr)
	}
}

func TestOrgStore_Update_ReparentsOrg(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	seedOrgUpdateWorkspace(t, db)

	store := orgAdapters.NewOrgStore(db)
	ctx := orgUpdateCtx()

	first, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "First Parent",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create first parent: %v", err)
	}

	second, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "Second Parent",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create second parent: %v", err)
	}

	child, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgUpdateWS,
		DisplayName: "Child",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	// Set initial parent.
	if _, err := db.Exec(
		`UPDATE organization SET parent_org_id=$1::uuid WHERE id=$2::uuid AND workspace_id=$3::uuid`,
		first.ID, child.ID, orgUpdateWS,
	); err != nil {
		t.Fatalf("seed initial parent: %v", err)
	}

	// Re-parent to second parent via Update.
	updated, err := store.Update(ctx, child.ID, orgUpdateWS, map[string]any{"parent_org_id": second.ID}, 0)
	if err != nil {
		t.Fatalf("reparent child: %v", err)
	}
	if updated.ParentOrgID == nil || *updated.ParentOrgID != second.ID {
		t.Fatalf("Update returned ParentOrgID=%v, want %s", updated.ParentOrgID, second.ID)
	}

	// Confirm via Get.
	fetched, err := store.Get(ctx, child.ID, orgUpdateWS)
	if err != nil {
		t.Fatalf("re-fetch child after reparent: %v", err)
	}
	if fetched.ParentOrgID == nil || *fetched.ParentOrgID != second.ID {
		t.Fatalf("Get after reparent returned ParentOrgID=%v, want %s", fetched.ParentOrgID, second.ID)
	}
}
