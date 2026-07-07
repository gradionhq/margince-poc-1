package workspacetx_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gradionhq/margince/backend/internal/platform/workspacetx"
)

// fakeExec records all ExecContext calls.
type fakeExec struct {
	queries []string
}

func (f *fakeExec) ExecContext(_ context.Context, q string, _ ...any) (sql.Result, error) {
	f.queries = append(f.queries, q)
	return &fakeResult{}, nil
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 0, nil }

func TestSetWorkspaceScope_CallsExpectedQueries(t *testing.T) {
	exec := &fakeExec{}
	err := workspacetx.SetWorkspaceScope(context.Background(), exec, "ws-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exec.queries) != 2 {
		t.Fatalf("expected 2 queries, got %d: %v", len(exec.queries), exec.queries)
	}
}
