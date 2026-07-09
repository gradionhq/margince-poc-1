//go:build integration

package adapters_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

const orgListWS = "00000000-0000-0000-0000-000000000053"

func seedOrgListWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		orgListWS, "org-list-ws", "org-list-ws-"+time.Now().Format("20060102150405"),
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

// TestOrgStore_List_IncludesParentOrgID is a regression test for the
// live-UAT-gate finding on RD-T09: List's SELECT (orgListColumns) and scan
// (scanOrgListRow) previously omitted parent_org_id, so every row returned
// by GET /organizations always reported parent_org_id=null regardless of
// what was actually stored — silently breaking the account-hierarchy tree,
// which groups List's rows by parent_org_id client-side. Get/GetAny already
// selected+scanned parent_org_id correctly; this is the read-side gap in
// the separate List query path.
func TestOrgStore_List_IncludesParentOrgID(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	seedOrgListWorkspace(t, db)

	store := orgAdapters.NewOrgStore(db)
	ctx := crmctx.With(t.Context(), crmctx.Principal{TenantID: orgListWS, UserID: "human:test"})

	parent, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgListWS,
		DisplayName: "List Parent Corp",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create parent org: %v", err)
	}

	child, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: orgListWS,
		DisplayName: "List Child Corp",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create child org: %v", err)
	}

	if _, err := store.Update(ctx, child.ID, orgListWS, map[string]any{"parent_org_id": parent.ID}, 0); err != nil {
		t.Fatalf("set child parent_org_id: %v", err)
	}

	rows, _, err := store.List(ctx, orgListWS, "", 200, "", orgDomain.OrgListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found bool
	for _, o := range rows {
		if o.ID != child.ID {
			continue
		}
		found = true
		if o.ParentOrgID == nil || *o.ParentOrgID != parent.ID {
			t.Fatalf("List row for child ParentOrgID=%v, want %s", o.ParentOrgID, parent.ID)
		}
	}
	if !found {
		t.Fatalf("List did not return the seeded child org %s", child.ID)
	}
}
