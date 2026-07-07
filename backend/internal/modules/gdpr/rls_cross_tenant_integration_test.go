//go:build integration

package crmgdpr_test

import (
	"context"
	"database/sql"
	"testing"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// TestGDPR_RLSBackstop proves the fix: person rows are visible only to their
// own tenant through the seam, even with a query carrying NO workspace_id
// predicate at all — the historical bug (WS-A) was that gdpr functions ran on
// the superuser pool, where FORCE RLS never engages, so only the hand-written
// predicate (present in this codebase's queries, but NOT a guarantee in
// general) stood between tenants.
func TestGDPR_RLSBackstop(t *testing.T) {
	db := testDB(t)
	wsA, personA := seedWorkspaceAndPerson(t, db)
	wsB, personB := seedWorkspaceAndPerson(t, db)
	_ = personB

	var countAsA int
	err := database.WithWorkspaceTx(context.Background(), db, wsA, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM person`).Scan(&countAsA)
	})
	if err != nil {
		t.Fatalf("query as tenant A: %v", err)
	}
	if countAsA != 1 {
		t.Errorf("tenant A should see exactly its own 1 person row with NO workspace_id predicate, got %d", countAsA)
	}

	var countAsB int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM person WHERE id = $1::uuid`, personA).Scan(&countAsB)
	})
	if err != nil {
		t.Fatalf("query as tenant B: %v", err)
	}
	if countAsB != 0 {
		t.Errorf("tenant B must see 0 rows for tenant A's specific person id even with no workspace_id predicate, got %d", countAsB)
	}
}
