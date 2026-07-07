//go:build integration

package transport

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedWorkspaceRoleAssignment creates a workspace, a role, a user, and assigns
// the role to the user via assignRoleTx itself (the real production path,
// now fixed — GH-209 WS-A#2), returning the workspace id.
func seedWorkspaceRoleAssignment(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	ws := ids.New()
	userID := ids.New()
	roleID := ids.New()

	mustExec(ctx, t, db,
		`INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		ws, "w"+ws, "w"+ws)
	mustExec(ctx, t, db,
		`INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		userID, ws, "u"+userID+"@example.com", "U")
	mustExec(ctx, t, db,
		`INSERT INTO role (id,workspace_id,key,is_system,permissions) VALUES ($1::uuid,$2::uuid,'rls-test',false,$3::jsonb)`,
		roleID, ws, `{}`)

	// assignRoleTx's audit write attributes the actor from ctx's crmctx.Principal
	// (EntryFromPrincipal) — carry one so WorkspaceID resolves for the audit row.
	principalCtx := crmctx.With(ctx, crmctx.Principal{TenantID: ws, UserID: userID})
	if err := assignRoleTx(principalCtx, db, ws, roleID, userID); err != nil {
		t.Fatalf("assignRoleTx: %v", err)
	}
	return ws
}

// TestIdentityTransport_RLSBackstop proves role_assignment rows are visible
// only to their own tenant through the seam, even with a query carrying NO
// workspace_id predicate at all (GH-209 WS-A cross-tenant proof).
func TestIdentityTransport_RLSBackstop(t *testing.T) {
	db, err := sql.Open("postgres", testDBURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	wsA := seedWorkspaceRoleAssignment(t, db)
	wsB := seedWorkspaceRoleAssignment(t, db)

	var countAsA int
	err = database.WithWorkspaceTx(context.Background(), db, wsA, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM role_assignment`).Scan(&countAsA)
	})
	if err != nil {
		t.Fatalf("query as tenant A: %v", err)
	}
	if countAsA != 1 {
		t.Errorf("tenant A should see exactly its own 1 role_assignment row with NO workspace_id predicate, got %d", countAsA)
	}

	var countAsB int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM role_assignment`).Scan(&countAsB)
	})
	if err != nil {
		t.Fatalf("query as tenant B: %v", err)
	}
	if countAsB != 1 {
		t.Errorf("tenant B should see exactly its own 1 role_assignment row with NO workspace_id predicate, got %d", countAsB)
	}
}
