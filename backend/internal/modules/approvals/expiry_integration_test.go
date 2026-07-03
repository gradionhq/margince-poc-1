//go:build integration

package crmapprovals_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	_ "github.com/lib/pq"
	"github.com/riverqueue/river"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// TestExpiryWorker covers AC-MCP-6: expired pending items are swept and marked expired.
func TestExpiryWorker(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	agentID := "agent:expiry-test-" + ids.New()
	payload := json.RawMessage(`{"kind":"person","id":"exp001","fields":{"email":"old@example.com"},"source":"mcp","captured_by":"` + agentID + `"}`)

	// Stage the item and commit so it persists.
	var itemID string
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin stage tx: %v", err)
	}
	itemID, stageErr := crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "update_record",
		RequestedBy: agentID,
		Payload:     payload,
	})
	if !errors.Is(stageErr, errs.ErrRequiresApproval) {
		_ = tx.Rollback()
		t.Fatalf("Stage: %v", stageErr)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}

	// Manually push expires_at into the past so the sweep picks it up.
	if _, err := db.ExecContext(context.Background(),
		`UPDATE approval_item SET expires_at = now() - interval '1 hour' WHERE id=$1::uuid`,
		itemID); err != nil {
		t.Fatalf("backdate expires_at: %v", err)
	}

	// Run the expiry sweep.
	worker := crmapprovals.NewExpiryWorker(db)
	if err := worker.Work(context.Background(), &river.Job[crmapprovals.ExpiryArgs]{}); err != nil {
		t.Fatalf("ExpiryWorker.Work: %v", err)
	}

	// Assert status = expired.
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		item, err := repo.Get(context.Background(), tx, itemID)
		if err != nil {
			t.Fatalf("Get after expiry: %v", err)
		}
		if item.Status != crmapprovals.StatusExpired {
			t.Errorf("status = %q, want expired", item.Status)
		}
	})

	// Assert audit_log row with action='expired'.
	var auditCount int
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			SELECT count(*) FROM audit_log
			WHERE workspace_id=$1::uuid AND action='expired'
			  AND entity_type='approval_item' AND entity_id=$2::uuid`,
			wsID, itemID).Scan(&auditCount)
		if err != nil {
			t.Fatalf("audit query: %v", err)
		}
	})
	if auditCount != 1 {
		t.Errorf("audit_log expired rows = %d, want 1", auditCount)
	}
}
