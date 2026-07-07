// Package database is the shared per-workspace connection-scoping seam
// (data-model §1.3): every statement touching a tenant table must run as the
// non-superuser margince_app role with app.workspace_id set, so FORCE ROW
// LEVEL SECURITY is actually engaged — the hand-written `WHERE workspace_id=$`
// predicate is defense in depth, never the sole backstop. Before this
// package, `withWorkspaceTx` was duplicated (byte-identical) in
// modules/directory and modules/deals, and entirely absent from gdpr,
// approvals, platform/audit, and identity/transport — those call sites
// silently ran on the superuser pool, bypassing RLS (GH-209 WS-A).
package database

import (
	"context"
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// DBExec is satisfied by both *sql.Tx and *sql.DB — the minimal exec surface
// SetWorkspaceScope needs. Deliberately narrow and local (mirrors
// crmaudit.DBExec / approvals.DBExec's existing same-shape interfaces in this
// codebase) so any package's own tx-carrying type structurally satisfies it
// with no import-time coupling.
type DBExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// WithWorkspaceTx begins a new tx, scopes it to workspaceID (SetWorkspaceScope),
// runs fn, and commits. Callers that own their own connection/tx lifecycle
// (the common case — gdpr, platform/audit, directory, deals) use this. fn
// must use the supplied tx for every statement, never the pool, or the
// role+GUC scoping is not in effect for that statement.
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

// SetWorkspaceScope runs the two per-workspace statements — SET LOCAL ROLE
// margince_app, then the app.workspace_id GUC — on an already-open exec. Use
// this directly (instead of WithWorkspaceTx) when the caller does not own the
// tx's lifecycle — e.g. approvals.Decider, which receives an already-open tx
// from its own caller and must not begin/commit one itself (GH-209 design
// deviation D1).
//
// Best-effort also sets app.user_id from ctx's crmctx.Principal, when one is
// present, so RLS policies that need to know the acting principal (e.g.
// record_grant's widened backstop, GH-209 WS-B) can reference
// current_setting('app.user_id', true) without every call site needing to
// change. Absent a principal on ctx (e.g. a background job), app.user_id
// stays unset — any policy referencing it degrades safely to "no match",
// never a regression for call sites that never carried a principal.
func SetWorkspaceScope(ctx context.Context, exec DBExec, workspaceID string) error {
	if _, err := exec.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return err
	}
	if _, err := exec.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return err
	}
	if p, ok := crmctx.From(ctx); ok && p.UserID != "" {
		if _, err := exec.ExecContext(ctx, `SELECT set_config('app.user_id', $1, true)`, p.UserID); err != nil {
			return err
		}
	}
	return nil
}
