//go:build integration

package crmapprovals_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	_ "github.com/lib/pq"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestRepository_SetAndGetResumeWindow_Integration(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.ExecContext(context.Background(),
		`SELECT set_config('app.workspace_id', $1, true)`, wsID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("set guc: %v", err)
	}

	id, err := repo.Create(context.Background(), tx, crmapprovals.Item{
		WorkspaceID: wsID,
		ActionType:  "update_record",
		Payload:     json.RawMessage(`{"id":"r1"}`),
		Status:      crmapprovals.StatusPending,
		RequestedBy: "agent1",
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("create: %v", err)
	}

	window := json.RawMessage(`{"goal":"fix r1","cursor":3,"observations":["a","b"],"pending":{"tool":"update_record","args":{"id":"r1"}}}`)
	if err := repo.SetResumeWindow(context.Background(), tx, id, window); err != nil {
		_ = tx.Rollback()
		t.Fatalf("setwindow: %v", err)
	}

	got, err := repo.Get(context.Background(), tx, id)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("get: %v", err)
	}
	_ = tx.Rollback()

	// resume_window is a jsonb column: Postgres canonicalizes key order and
	// whitespace, so compare semantically (unmarshal + DeepEqual), not byte-for-byte.
	var gotVal, wantVal any
	if err := json.Unmarshal(got.ResumeWindow, &gotVal); err != nil {
		t.Fatalf("unmarshal got: %v (raw=%s)", err, got.ResumeWindow)
	}
	if err := json.Unmarshal(window, &wantVal); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(gotVal, wantVal) {
		t.Fatalf("window = %s, want (semantically) %s", got.ResumeWindow, window)
	}
}
