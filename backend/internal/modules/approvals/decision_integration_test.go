//go:build integration

package crmapprovals_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	_ "github.com/lib/pq"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// testSorProvider is a spy that implements datasource.Provider for tests.
type testSorProvider struct {
	lastUpdateCmd *datasource.UpdateInput
	lastCreateCmd *datasource.CreateInput
}

func (s *testSorProvider) Read(ctx context.Context, ref datasource.EntityRef) (any, error) {
	return map[string]any{"type": string(ref.Type), "id": ref.ID, "email": "old@example.com"}, nil
}

func (s *testSorProvider) Search(ctx context.Context, query datasource.SearchQuery) (datasource.SearchResult, error) {
	return datasource.SearchResult{}, nil
}

func (s *testSorProvider) ListObjects(ctx context.Context) ([]datasource.ObjectDef, error) {
	return nil, nil
}

func (s *testSorProvider) ListFields(ctx context.Context, t datasource.EntityType) ([]datasource.FieldDef, error) {
	return nil, nil
}

func (s *testSorProvider) Create(ctx context.Context, in datasource.CreateInput) (datasource.EntityRef, error) {
	s.lastCreateCmd = &in
	return datasource.EntityRef{Type: in.Type, ID: "new-id"}, nil
}

func (s *testSorProvider) Update(ctx context.Context, in datasource.UpdateInput) (datasource.EntityRef, error) {
	s.lastUpdateCmd = &in
	return datasource.EntityRef{Type: in.Type, ID: in.ID}, nil
}

func (s *testSorProvider) AdvanceDeal(ctx context.Context, in datasource.AdvanceDealInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{Type: datasource.EntityDeal, ID: in.DealID}, nil
}

func (s *testSorProvider) RunReport(ctx context.Context, plan datasource.ReportPlan) (datasource.ReportResult, error) {
	//nolint:nilnil // fake provider: a nil report with no error is a valid empty result
	return nil, nil
}

func (s *testSorProvider) Freshness(ctx context.Context, ref datasource.EntityRef) (datasource.FreshnessInfo, error) {
	return datasource.FreshnessInfo{Authoritative: true}, nil
}

func (s *testSorProvider) LinkConversation(ctx context.Context, in datasource.LinkConversationInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{Type: datasource.EntityType(in.EntityType), ID: "link-id"}, nil
}

func (s *testSorProvider) UnlinkConversation(ctx context.Context, in datasource.UnlinkConversationInput) error {
	return nil
}

// TestComputePreview covers AC-MCP-4: preview must return before/after without mutating.
func TestComputePreview(t *testing.T) {
	provider := &testSorProvider{}
	ctx := context.Background()

	payload := json.RawMessage(`{"kind":"person","id":"abc123","fields":{"email":"new@example.com"}}`)

	result, err := crmapprovals.ComputePreview(ctx, provider, "update_record", payload)
	if err != nil {
		t.Fatalf("ComputePreview: %v", err)
	}

	var preview map[string]any
	if err := json.Unmarshal(result, &preview); err != nil {
		t.Fatalf("unmarshal preview: %v", err)
	}

	if _, ok := preview["before"]; !ok {
		t.Error("preview missing 'before' key")
	}
	if _, ok := preview["after"]; !ok {
		t.Error("preview missing 'after' key")
	}

	// No datasource mutations occurred.
	if provider.lastUpdateCmd != nil {
		t.Error("ComputePreview must not call datasource.Update")
	}
	if provider.lastCreateCmd != nil {
		t.Error("ComputePreview must not call datasource.Create")
	}
}

// TestApproveDecision covers AC-MCP-5: approve sets status, writes audit, calls datasource.
func TestApproveDecision(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	provider := &testSorProvider{}
	decider := crmapprovals.Decider{Repo: repo, Datasource: provider}

	agentID := "agent:approver-test-" + ids.New()
	payload := json.RawMessage(`{"kind":"person","id":"abc123","fields":{"email":"new@example.com"},"source":"mcp","captured_by":"` + agentID + `"}`)

	// Stage the item and commit.
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

	// Approve in a new tx.
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin approve tx: %v", err)
	}
	if _, err := tx2.ExecContext(context.Background(),
		`SELECT set_config('app.workspace_id', $1, true)`, wsID); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("set guc approve: %v", err)
	}
	if err := decider.Approve(context.Background(), tx2, itemID, "human:approver-1"); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("Approve: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit approve: %v", err)
	}

	// Assert status = approved.
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		item, err := repo.Get(context.Background(), tx, itemID)
		if err != nil {
			t.Fatalf("Get after approve: %v", err)
		}
		if item.Status != crmapprovals.StatusApproved {
			t.Errorf("status = %q, want approved", item.Status)
		}
	})

	// Assert audit_log row with action='approve'.
	var auditCount int
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			SELECT count(*) FROM audit_log
			WHERE workspace_id=$1::uuid AND action='approve'
			  AND entity_type='approval_item' AND entity_id=$2::uuid`,
			wsID, itemID).Scan(&auditCount)
		if err != nil {
			t.Fatalf("audit query: %v", err)
		}
	})
	if auditCount != 1 {
		t.Errorf("audit_log approve rows = %d, want 1", auditCount)
	}

	// Assert datasource.Update was called.
	if provider.lastUpdateCmd == nil {
		t.Error("Approve must call datasource.Update")
	}
}

// TestRejectDecision: reject sets status, writes audit, does NOT call datasource.
func TestRejectDecision(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	provider := &testSorProvider{}
	decider := crmapprovals.Decider{Repo: repo, Datasource: provider}

	agentID := "agent:reject-test-" + ids.New()
	payload := json.RawMessage(`{"kind":"person","id":"abc999","fields":{"email":"bad@example.com"},"source":"mcp","captured_by":"` + agentID + `"}`)

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

	// Reject in a new tx.
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin reject tx: %v", err)
	}
	if _, err := tx2.ExecContext(context.Background(),
		`SELECT set_config('app.workspace_id', $1, true)`, wsID); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("set guc reject: %v", err)
	}
	if err := decider.Reject(context.Background(), tx2, itemID, "human:approver-1", "not appropriate"); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("Reject: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit reject: %v", err)
	}

	// Assert status = rejected.
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		item, err := repo.Get(context.Background(), tx, itemID)
		if err != nil {
			t.Fatalf("Get after reject: %v", err)
		}
		if item.Status != crmapprovals.StatusRejected {
			t.Errorf("status = %q, want rejected", item.Status)
		}
	})

	// Assert audit_log row with action='reject'.
	var auditCount int
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			SELECT count(*) FROM audit_log
			WHERE workspace_id=$1::uuid AND action='reject'
			  AND entity_type='approval_item' AND entity_id=$2::uuid`,
			wsID, itemID).Scan(&auditCount)
		if err != nil {
			t.Fatalf("audit query: %v", err)
		}
	})
	if auditCount != 1 {
		t.Errorf("audit_log reject rows = %d, want 1", auditCount)
	}

	// Datasource must NOT be called.
	if provider.lastUpdateCmd != nil {
		t.Error("Reject must not call datasource.Update")
	}
	if provider.lastCreateCmd != nil {
		t.Error("Reject must not call datasource.Create")
	}
}

// TestModifyDecision: modify sets status, calls datasource, writes 2 audit rows; gate rejection leaves pending.
func TestModifyDecision(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	provider := &testSorProvider{}
	decider := crmapprovals.Decider{Repo: repo, Datasource: provider}

	agentID := "agent:modify-test-" + ids.New()
	payload := json.RawMessage(`{"kind":"person","id":"abc456","fields":{"email":"old@example.com"},"source":"mcp","captured_by":"` + agentID + `"}`)

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

	editedPayload := json.RawMessage(`{"kind":"person","id":"abc456","fields":{"email":"edited@example.com"},"source":"mcp","captured_by":"` + agentID + `"}`)

	passAdmit := func(ctx context.Context, approverID string, actionType string, p json.RawMessage) error {
		return nil
	}

	// Modify in a new tx.
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin modify tx: %v", err)
	}
	if _, err := tx2.ExecContext(context.Background(),
		`SELECT set_config('app.workspace_id', $1, true)`, wsID); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("set guc modify: %v", err)
	}
	if err := decider.Modify(context.Background(), tx2, itemID, "human:approver-1", editedPayload, passAdmit); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("Modify: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit modify: %v", err)
	}

	// Assert status = modified.
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		item, err := repo.Get(context.Background(), tx, itemID)
		if err != nil {
			t.Fatalf("Get after modify: %v", err)
		}
		if item.Status != crmapprovals.StatusModified {
			t.Errorf("status = %q, want modified", item.Status)
		}
	})

	// Assert two audit rows with action='modify'.
	var auditCount int
	withGUC(t, db, wsID, func(tx *sql.Tx) {
		err := tx.QueryRowContext(context.Background(), `
			SELECT count(*) FROM audit_log
			WHERE workspace_id=$1::uuid AND action='modify'
			  AND entity_type='approval_item' AND entity_id=$2::uuid`,
			wsID, itemID).Scan(&auditCount)
		if err != nil {
			t.Fatalf("audit query: %v", err)
		}
	})
	if auditCount != 2 {
		t.Errorf("audit_log modify rows = %d, want 2", auditCount)
	}

	// Datasource must have been called with edited payload.
	if provider.lastUpdateCmd == nil {
		t.Fatal("Modify must call datasource.Update")
	}
	patch, ok := provider.lastUpdateCmd.Patch.(map[string]any)
	if !ok {
		t.Fatalf("UpdateInput.Patch is not map[string]any, got %T", provider.lastUpdateCmd.Patch)
	}
	if patch["email"] != "edited@example.com" {
		t.Errorf("datasource update email = %v, want edited@example.com", patch["email"])
	}

	// Sub-test: gate rejects → error, status stays pending.
	t.Run("gate_rejects", func(t *testing.T) {
		db2 := testDB(t)
		wsID2 := seedWorkspace(t, db2)
		repo2 := crmapprovals.NewRepository()
		provider2 := &testSorProvider{}
		decider2 := crmapprovals.Decider{Repo: repo2, Datasource: provider2}

		agentID2 := "agent:gate-test-" + ids.New()
		payload2 := json.RawMessage(`{"kind":"person","id":"abc789","fields":{"email":"x@x.com"},"source":"mcp","captured_by":"` + agentID2 + `"}`)

		tx3, err := db2.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatalf("begin stage tx: %v", err)
		}
		itemID2, stageErr2 := crmapprovals.Stage(context.Background(), tx3, repo2, crmapprovals.StageInput{
			WorkspaceID: wsID2,
			ActionType:  "update_record",
			RequestedBy: agentID2,
			Payload:     payload2,
		})
		if !errors.Is(stageErr2, errs.ErrRequiresApproval) {
			_ = tx3.Rollback()
			t.Fatalf("Stage: %v", stageErr2)
		}
		if err := tx3.Commit(); err != nil {
			t.Fatalf("commit stage: %v", err)
		}

		rejectGate := func(ctx context.Context, approverID string, actionType string, p json.RawMessage) error {
			return errs.ErrScopeExceeded
		}

		tx4, err := db2.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatalf("begin modify tx: %v", err)
		}
		if _, err := tx4.ExecContext(context.Background(),
			`SELECT set_config('app.workspace_id', $1, true)`, wsID2); err != nil {
			_ = tx4.Rollback()
			t.Fatalf("set guc: %v", err)
		}
		modErr := decider2.Modify(context.Background(), tx4, itemID2, "human:approver-1", editedPayload, rejectGate)
		_ = tx4.Rollback()

		if modErr == nil {
			t.Error("Modify with rejecting gate should return an error")
		}
		if !errors.Is(modErr, errs.ErrScopeExceeded) {
			t.Errorf("error = %v, want ErrScopeExceeded", modErr)
		}

		// Status must still be pending.
		withGUC(t, db2, wsID2, func(tx *sql.Tx) {
			item, err := repo2.Get(context.Background(), tx, itemID2)
			if err != nil {
				t.Fatalf("Get after rejected gate: %v", err)
			}
			if item.Status != crmapprovals.StatusPending {
				t.Errorf("status after gate rejection = %q, want pending", item.Status)
			}
		})

		// Datasource must not have been called.
		if provider2.lastUpdateCmd != nil {
			t.Error("datasource.Update must not be called when gate rejects")
		}
	})
}
