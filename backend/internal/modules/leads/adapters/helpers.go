// Package adapters contains the leads module's PostgreSQL storage adapters.
package adapters

import (
	"context"
	"database/sql"
)

// withWorkspaceTx runs fn inside a single tx as the non-superuser margince_app
// role with app.workspace_id set, so FORCE RLS is actually enforced on every
// CRUD query (data-model §1.3).
func withWorkspaceTx(ctx context.Context, db *sql.DB, workspaceID string, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Generic, domain-free store helpers (provenance guard, keyset cursors,
// bounded-update field readers) live in the Tier-0 shared/kernel/sqlutil package.
