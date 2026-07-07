//go:build integration

package crmaudit_test

import (
	"context"
	"database/sql"
	"testing"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedWorkspaceAndAuditRow creates a workspace and writes one audit_log row
// via crmaudit.Write itself (now fixed, GH-209 WS-A#2), returning the
// workspace id.
func seedWorkspaceAndAuditRow(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		wsID, "w"+wsID, "w"+wsID)
	entID := ids.New()
	if _, err := crmaudit.Write(context.Background(), db, crmaudit.Entry{
		WorkspaceID: wsID,
		ActorType:   "system",
		ActorID:     "system",
		Action:      "create",
		EntityType:  "person",
		EntityID:    &entID,
	}); err != nil {
		t.Fatalf("seed audit row: %v", err)
	}
	return wsID
}

// TestAudit_RLSBackstop proves audit_log rows are visible only to their own
// tenant through the seam, even with a query carrying NO workspace_id
// predicate at all (GH-209 WS-A cross-tenant proof).
func TestAudit_RLSBackstop(t *testing.T) {
	db := testDB(t)
	wsA := seedWorkspaceAndAuditRow(t, db)
	wsB := seedWorkspaceAndAuditRow(t, db)

	var countAsA int
	err := database.WithWorkspaceTx(context.Background(), db, wsA, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM audit_log`).Scan(&countAsA)
	})
	if err != nil {
		t.Fatalf("query as tenant A: %v", err)
	}
	if countAsA != 1 {
		t.Errorf("tenant A should see exactly its own 1 audit_log row with NO workspace_id predicate, got %d", countAsA)
	}

	var countAsB int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM audit_log`).Scan(&countAsB)
	})
	if err != nil {
		t.Fatalf("query as tenant B: %v", err)
	}
	if countAsB != 1 {
		t.Errorf("tenant B should see exactly its own 1 audit_log row with NO workspace_id predicate, got %d", countAsB)
	}
}
