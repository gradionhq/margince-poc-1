package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// fakeExec is a minimal database.DBExec recorder — no real DB needed for these
// two unit tests; the integration proof that the statements actually engage
// RLS lives in Task 4's cross-tenant integration tests (which need a live PG).
type fakeExec struct {
	stmts []string
	args  [][]any
	fail  string // if set, ExecContext with this exact query returns an error
}

func (f *fakeExec) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.stmts = append(f.stmts, query)
	f.args = append(f.args, args)
	if f.fail != "" && query == f.fail {
		return nil, errors.New("boom")
	}
	return nil, nil
}

func TestSetWorkspaceScope_RunsRoleThenGUC(t *testing.T) {
	exec := &fakeExec{}
	if err := database.SetWorkspaceScope(context.Background(), exec, "ws-1"); err != nil {
		t.Fatalf("SetWorkspaceScope: %v", err)
	}
	if len(exec.stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %v", len(exec.stmts), exec.stmts)
	}
	if exec.stmts[0] != `SET LOCAL ROLE margince_app` {
		t.Errorf("statement 1 = %q, want role switch first", exec.stmts[0])
	}
	if exec.stmts[1] != `SELECT set_config('app.workspace_id', $1, true)` {
		t.Errorf("statement 2 = %q, want workspace GUC", exec.stmts[1])
	}
	if exec.args[1][0] != "ws-1" {
		t.Errorf("workspace GUC arg = %v, want ws-1", exec.args[1])
	}
}

func TestSetWorkspaceScope_PropagatesRoleError(t *testing.T) {
	exec := &fakeExec{fail: `SET LOCAL ROLE margince_app`}
	if err := database.SetWorkspaceScope(context.Background(), exec, "ws-1"); err == nil {
		t.Fatal("want error when SET LOCAL ROLE fails, got nil")
	}
	// The GUC statement must never run if the role switch failed.
	if len(exec.stmts) != 1 {
		t.Errorf("want exactly 1 statement attempted (role only), got %d: %v", len(exec.stmts), exec.stmts)
	}
}

func TestSetWorkspaceScope_SetsUserIDFromContext(t *testing.T) {
	exec := &fakeExec{}
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "user-1", TenantID: "ws-1"})
	if err := database.SetWorkspaceScope(ctx, exec, "ws-1"); err != nil {
		t.Fatalf("SetWorkspaceScope: %v", err)
	}
	if len(exec.stmts) != 3 {
		t.Fatalf("want 3 statements (role, workspace GUC, user GUC), got %d: %v", len(exec.stmts), exec.stmts)
	}
	if exec.stmts[2] != `SELECT set_config('app.user_id', $1, true)` {
		t.Errorf("statement 3 = %q, want user GUC", exec.stmts[2])
	}
	if exec.args[2][0] != "user-1" {
		t.Errorf("user GUC arg = %v, want user-1", exec.args[2])
	}
}

func TestSetWorkspaceScope_NoUserIDWhenNoPrincipal(t *testing.T) {
	exec := &fakeExec{}
	if err := database.SetWorkspaceScope(context.Background(), exec, "ws-1"); err != nil {
		t.Fatalf("SetWorkspaceScope: %v", err)
	}
	if len(exec.stmts) != 2 {
		t.Fatalf("want 2 statements (no principal on ctx -> no user GUC), got %d: %v", len(exec.stmts), exec.stmts)
	}
}

// dbFromEnvOrSkip is intentionally NOT defined here — WithWorkspaceTx's
// tx-lifecycle behavior (begin/commit/rollback against a real *sql.DB) is
// proven by the existing callers' own integration tests (directory/deals
// already exercise withWorkspaceTx end-to-end) plus Task 4's new cross-tenant
// integration tests. These two unit tests cover the seam's own new logic
// (SetWorkspaceScope's statement order/error-propagation/user-GUC behavior)
// without needing a live Postgres.
