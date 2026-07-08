// Package customfields (this file): SetOptions — the one lifecycle mutation
// besides Create that runs DDL (regenerating a picklist column's CHECK), so
// it follows Create's base-role-ALTER-then-downgrade tx shape (see
// create.go's role-switch note: margince_app has no ALTER privilege).
package customfields

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// ErrNotPicklist is returned when SetOptions targets a field whose type is
// not 'picklist' — only a picklist has an options-derived CHECK to regenerate.
var ErrNotPicklist = errors.New("customfields: not a picklist field")

// ErrLastOption is returned when the requested options list is empty — a
// picklist must always keep at least one allowed value.
var ErrLastOption = errors.New("customfields: a picklist needs at least one option")

// SetOptions replaces a picklist field's allowed option set and regenerates
// the physical column's CHECK constraint from it (CUSTOM-FIELDS-PARAM-5).
// Refuses a non-picklist field (ErrNotPicklist) and an empty options list
// (ErrLastOption) before touching the database. Runs the ALTER TABLE as the
// pool's base role, then downgrades to margince_app for the catalog UPDATE
// + one audit row — all in one tx, mirroring Create (create.go).
func SetOptions(ctx context.Context, db *sql.DB, id string, options []string) (Created, error) {
	if len(options) == 0 {
		return Created{}, ErrLastOption
	}
	p, _ := crmctx.From(ctx)
	if p.TenantID == "" {
		return Created{}, fmt.Errorf("customfields: empty workspace_id")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Created{}, fmt.Errorf("customfields: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var object, columnName, fieldType string
	var oldOptionsRaw []byte
	if err := tx.QueryRowContext(ctx, `SELECT object, column_name, type, options FROM custom_field WHERE id=$1::uuid AND workspace_id=$2::uuid FOR UPDATE`,
		id, p.TenantID).Scan(&object, &columnName, &fieldType, &oldOptionsRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Created{}, ErrNotFound
		}
		return Created{}, fmt.Errorf("customfields: select for options edit: %w", err)
	}
	if fieldType != TypePicklist {
		return Created{}, ErrNotPicklist
	}

	ddl, err := BuildOptionsDDL(object, columnName, options)
	if err != nil {
		return Created{}, err
	}
	// Step 1: regenerate the CHECK as the pool's base (owning) role.
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return Created{}, fmt.Errorf("customfields: alter check constraint: %w", err)
	}

	// Step 2: downgrade to margince_app + set the workspace GUC so the
	// catalog update and audit write are RLS-governed like every other
	// tenant write.
	if err := database.SetWorkspaceScope(ctx, tx, p.TenantID); err != nil {
		return Created{}, fmt.Errorf("customfields: set workspace scope: %w", err)
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return Created{}, fmt.Errorf("customfields: marshal options: %w", err)
	}
	row := tx.QueryRowContext(ctx, `UPDATE custom_field SET options=$1 WHERE id=$2::uuid AND workspace_id=$3::uuid RETURNING `+catalogColumns,
		optionsJSON, id, p.TenantID)
	out, err := scanCatalogRow(row)
	if err != nil {
		return Created{}, fmt.Errorf("customfields: update options: %w", err)
	}

	var oldOptions []string
	if len(oldOptionsRaw) > 0 {
		_ = json.Unmarshal(oldOptionsRaw, &oldOptions)
	}
	entID := out.ID
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.EntryFromPrincipal(ctx, "update", "custom_field", &entID,
		map[string]any{"options": oldOptions}, map[string]any{"options": options})); err != nil {
		return Created{}, fmt.Errorf("customfields: audit write: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Created{}, fmt.Errorf("customfields: commit: %w", err)
	}
	return out, nil
}
