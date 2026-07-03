//go:build integration

package crmapprovals_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"

	_ "github.com/lib/pq"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedWorkspace(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		wsID, "approvals-"+wsID, "approvals-"+wsID,
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return wsID
}

// withGUC runs fn inside a transaction with app.workspace_id set.
func withGUC(t *testing.T, db *sql.DB, wsID string, fn func(*sql.Tx)) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(context.Background(),
		`SELECT set_config('app.workspace_id', $1, true)`, wsID); err != nil {
		t.Fatalf("set guc: %v", err)
	}
	fn(tx)
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// TestStageCommitBlock covers AC-MCP-2 and AC-MCP-3.
func TestStageCommitBlock(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	payload := json.RawMessage(`{"action_type":"update_record","entity_id":"abc"}`)
	agentID := "agent:test-passport-" + ids.New()

	// Stage the action in a tx, then commit so the rows persist.
	var itemID string
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	itemID, stageErr := crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "update_record",
		RequestedBy: agentID,
		Payload:     payload,
	})
	if !errors.Is(stageErr, errs.ErrRequiresApproval) {
		_ = tx.Rollback()
		t.Fatalf("Stage should return ErrRequiresApproval, got: %v", stageErr)
	}
	if itemID == "" {
		_ = tx.Rollback()
		t.Fatal("Stage should return a non-empty item ID")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Assert one approval_item row with status=pending and correct fields.
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		item, err := repo.Get(context.Background(), tx, itemID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if item.Status != crmapprovals.StatusPending {
			t.Errorf("status = %q, want pending", item.Status)
		}
		if item.ActionType != "update_record" {
			t.Errorf("action_type = %q, want update_record", item.ActionType)
		}
		if item.RequestedBy != agentID {
			t.Errorf("requested_by = %q, want %q", item.RequestedBy, agentID)
		}
		if item.ExpiresAt == nil {
			t.Error("expires_at is nil, want non-null")
		}
		// payload round-trips
		var got map[string]any
		if err := json.Unmarshal(item.Payload, &got); err != nil {
			t.Fatalf("payload unmarshal: %v", err)
		}
		if got["action_type"] != "update_record" {
			t.Errorf("payload round-trip: action_type = %v", got["action_type"])
		}
	})

	// Assert one audit_log row with pending_approval state.
	var auditCount int
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			SELECT count(*) FROM audit_log
			WHERE workspace_id=$1::uuid
			  AND action='capture'
			  AND entity_type='approval_item'
			  AND entity_id=$2::uuid`,
			wsID, itemID).Scan(&auditCount)
		if err != nil {
			t.Fatalf("audit_log query: %v", err)
		}
	})
	if auditCount != 1 {
		t.Errorf("audit_log rows = %d, want 1", auditCount)
	}

	// Assert no row was written to person (zero side effects on primary table).
	var personCount int
	if err := db.QueryRow(`SELECT count(*) FROM person WHERE workspace_id=$1::uuid`, wsID).Scan(&personCount); err != nil {
		t.Fatalf("person count: %v", err)
	}
	if personCount != 0 {
		t.Errorf("person rows = %d, want 0 (Stage must not write to primary table)", personCount)
	}

	// RLS: from a different workspace GUC, ListByStatus returns zero rows.
	otherWsID := ids.New()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		otherWsID, "other-"+otherWsID, "other-"+otherWsID,
	); err != nil {
		t.Fatalf("seed other workspace: %v", err)
	}
	withGUC(t, db, otherWsID, func(tx *sql.Tx) {
		// Switch to the non-superuser role so FORCE RLS is enforced.
		if _, err := tx.ExecContext(context.Background(), `SET LOCAL ROLE margince_app`); err != nil {
			t.Fatalf("set role: %v", err)
		}
		items, err := repo.ListByStatus(context.Background(), tx, wsID, crmapprovals.StatusPending)
		if err != nil {
			t.Fatalf("ListByStatus cross-ws: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("cross-workspace isolation failed: got %d items, want 0", len(items))
		}
	})
}
