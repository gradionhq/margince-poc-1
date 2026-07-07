//go:build integration

package crmauth_test

import (
	"context"
	"database/sql"
	"testing"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// TestSessionPassport_RLSBackstop proves GH-209's escalation fold-in on the
// live-production auth path: session and passport rows are visible only to
// their own tenant through the seam, even with a query carrying NO
// workspace_id predicate at all — SessionStore.Create/Touch/Delete and
// PassportStore.Create/Revoke now route through platform/database
// (Option 1); only Lookup (both stores) remains a reasoned rls-exempt,
// resolving workspace_id from the opaque token itself, never from a
// tenant-scoped connection.
func TestSessionPassport_RLSBackstop(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	wsA := seedWorkspace(t, db)
	wsB := seedWorkspace(t, db)
	userA := seedUser(t, db, wsA)
	userB := seedUser(t, db, wsB)

	sessions := crmauth.NewSessionStore(db)
	if _, err := sessions.Create(ctx, wsA, userA); err != nil {
		t.Fatalf("create session A: %v", err)
	}
	if _, err := sessions.Create(ctx, wsB, userB); err != nil {
		t.Fatalf("create session B: %v", err)
	}

	var countAsA int
	err := database.WithWorkspaceTx(ctx, db, wsA, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM session`).Scan(&countAsA)
	})
	if err != nil {
		t.Fatalf("query session as tenant A: %v", err)
	}
	if countAsA != 1 {
		t.Errorf("tenant A should see exactly its own 1 session row with NO workspace_id predicate, got %d", countAsA)
	}

	passports := crmauth.NewPassportStore(db)
	if _, _, err := passports.Create(ctx, wsA, userA, []string{"read"}, 3600e9); err != nil {
		t.Fatalf("create passport A: %v", err)
	}
	if _, _, err := passports.Create(ctx, wsB, userB, []string{"read"}, 3600e9); err != nil {
		t.Fatalf("create passport B: %v", err)
	}

	var countPassportsAsB int
	err = database.WithWorkspaceTx(ctx, db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM passport`).Scan(&countPassportsAsB)
	})
	if err != nil {
		t.Fatalf("query passport as tenant B: %v", err)
	}
	if countPassportsAsB != 1 {
		t.Errorf("tenant B should see exactly its own 1 passport row with NO workspace_id predicate, got %d", countPassportsAsB)
	}
}
