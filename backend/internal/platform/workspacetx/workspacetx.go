// Package workspacetx provides the shared workspace-scoped transaction helper
// used by every Tier-1 domain module's adapters layer (WS-E-b, D7).
// This is a deliberate, temporary pre-aligned duplicate of GH-209's
// platform/database seam — Task 8 will fold this into platform/database once
// GH-209 merges to main, via a pure import-path swap.
package workspacetx

import (
	"context"
	"database/sql"
)

// DBExec is satisfied by *sql.DB, *sql.Tx, and any test fake.
type DBExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SetWorkspaceScope sets the app.workspace_id GUC on exec so that FORCE ROW
// LEVEL SECURITY policies see the right tenant. exec is typically a *sql.Tx
// begun by WithWorkspaceTx.
func SetWorkspaceScope(ctx context.Context, exec DBExec, workspaceID string) error {
	if _, err := exec.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return err
	}
	_, err := exec.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID)
	return err
}

// WithWorkspaceTx runs fn inside a single tx as the non-superuser margince_app
// role with app.workspace_id set, so FORCE RLS is actually enforced on every
// CRUD query (data-model §1.3). fn must use the supplied tx (never the pool)
// so the role+GUC are in scope for its statements.
func WithWorkspaceTx(ctx context.Context, db *sql.DB, workspaceID string, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := SetWorkspaceScope(ctx, tx, workspaceID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
