//go:build integration

package crmapprovals_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// seedApprovalItem stages one pending approval_item in wsID via the real
// crmapprovals.Stage production path, run inside the workspace-scoping seam.
func seedApprovalItem(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	repo := crmapprovals.NewRepository()
	err := database.WithWorkspaceTx(context.Background(), db, wsID, func(tx *sql.Tx) error {
		_, stageErr := crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
			WorkspaceID: wsID,
			ActionType:  "update_record",
			RequestedBy: "agent:test-passport-" + wsID,
			Payload:     json.RawMessage(`{"kind":"deal"}`),
		})
		if !errors.Is(stageErr, errs.ErrRequiresApproval) {
			return stageErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed approval_item: %v", err)
	}
}

// TestApprovals_RLSBackstop proves approval_item rows are visible only to
// their own tenant through the seam, even with a query carrying NO
// workspace_id predicate at all (GH-209 WS-A cross-tenant proof).
func TestApprovals_RLSBackstop(t *testing.T) {
	db := testDB(t)
	wsA := seedWorkspace(t, db)
	wsB := seedWorkspace(t, db)
	seedApprovalItem(t, db, wsA)
	seedApprovalItem(t, db, wsB)

	var countAsA int
	err := database.WithWorkspaceTx(context.Background(), db, wsA, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM approval_item`).Scan(&countAsA)
	})
	if err != nil {
		t.Fatalf("query as tenant A: %v", err)
	}
	if countAsA != 1 {
		t.Errorf("tenant A should see exactly its own 1 approval_item row with NO workspace_id predicate, got %d", countAsA)
	}

	var countAsB int
	err = database.WithWorkspaceTx(context.Background(), db, wsB, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM approval_item`).Scan(&countAsB)
	})
	if err != nil {
		t.Fatalf("query as tenant B: %v", err)
	}
	if countAsB != 1 {
		t.Errorf("tenant B should see exactly its own 1 approval_item row with NO workspace_id predicate, got %d", countAsB)
	}
}
