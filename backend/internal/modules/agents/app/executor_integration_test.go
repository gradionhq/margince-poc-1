//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestHandleDecided_ApprovedExecutesAndAudits(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	// Stage a fixture 🟡 proposal directly (bypassing app.StageProposal to
	// keep this test focused on the decided-event path).
	var itemID string
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	itemID, _ = crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "overnight.log_link",
		RequestedBy: app.ActorOvernight,
		Payload:     json.RawMessage(`{"link":"call-123"}`),
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}

	// Approve it through the REAL approvals Decider — proves the
	// integration point (Decider.execAction no-ops for "overnight.log_link",
	// "delegated to tool handlers"; our HandleDecided is that handler).
	decider := crmapprovals.Decider{Repo: repo, Datasource: nil, Emitter: nil}
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin approve: %v", err)
	}
	if err := decider.Approve(context.Background(), tx2, itemID, "human-approver-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit approve: %v", err)
	}

	// Now consume the (would-be) approval.decided event: our executor.
	effector := &spyEffector{rollbackHandle: "rb-2"}
	emitter := &spyEmitter{}
	tx3, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx3, repo, effector, emitter, app.DecidedEventPayload{Decision: "approved", ItemID: itemID}); err != nil {
		t.Fatalf("HandleDecided: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("commit handle: %v", err)
	}

	if !effector.called {
		t.Fatal("effector.Apply was never called for an approved overnight item")
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE workspace_id = $1::uuid AND actor_id = $2 AND action = 'update'`, wsID, app.ActorOvernight).Scan(&count); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("agent:overnight audit rows = %d, want 1 (separate from the approvals module's own 'approve' row)", count)
	}
}

func TestHandleDecided_RejectedExecutesNothing(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	var itemID string
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	itemID, _ = crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID, ActionType: "overnight.log_link", RequestedBy: app.ActorOvernight, Payload: json.RawMessage(`{}`),
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}

	decider := crmapprovals.Decider{Repo: repo}
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin reject: %v", err)
	}
	if err := decider.Reject(context.Background(), tx2, itemID, "human-approver-1", "not needed"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit reject: %v", err)
	}

	effector := &spyEffector{}
	tx3, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx3, repo, effector, &spyEmitter{}, app.DecidedEventPayload{Decision: "rejected", ItemID: itemID}); err != nil {
		t.Fatalf("HandleDecided: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("commit handle: %v", err)
	}

	if effector.called {
		t.Fatal("effector.Apply must not be called for a rejected item")
	}
}

func TestHandleDecided_IgnoresItemsOutsideItsNamespace(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	var itemID string
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	itemID, _ = crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID, ActionType: "some_other_module.action", RequestedBy: "agent:other", Payload: json.RawMessage(`{}`),
	})
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}

	effector := &spyEffector{}
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx2, repo, effector, &spyEmitter{}, app.DecidedEventPayload{Decision: "approved", ItemID: itemID}); err != nil {
		t.Fatalf("HandleDecided: %v", err)
	}
	_ = tx2.Rollback()

	if effector.called {
		t.Fatal("effector.Apply must not be called for an item outside the overnight.* namespace")
	}
}
