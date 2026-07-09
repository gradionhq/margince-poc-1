package customfields

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/platform/database"
)

// List reads the custom_field catalog rows for one (workspace, object)
// (listCustomFields, CUSTOM-FIELDS-WIRE-1 — the admin field-table read behind
// the custom-fields admin screen). Unlike a normal resource list this admin
// view returns both active AND retired rows by default; a non-empty status
// ("active" or "retired") narrows to that one lifecycle state. Ordered
// oldest-first so the admin table is stable across reloads.
//
// The catalog is small and bounded per object, so this returns the full set
// in one read — cursor/limit pagination is intentionally not applied (the
// response's has_more is always false). It mirrors ActiveColumns' RLS-governed
// WithWorkspaceTx read pattern rather than Create's owning-role DDL path.
func List(ctx context.Context, db *sql.DB, workspaceID, object, status string) ([]Created, error) {
	fields := make([]Created, 0)
	err := database.WithWorkspaceTx(ctx, db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, object, slug, label, type, status, archived_at,
			       column_name, currency, options, created_by, created_at, updated_at, version
			FROM custom_field
			WHERE workspace_id=$1::uuid AND object=$2 AND ($3 = '' OR status=$3)
			ORDER BY created_at, id`, workspaceID, object, status)
		if err != nil {
			return fmt.Errorf("customfields: select catalog rows: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var c Created
			var optionsRaw []byte
			if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Object, &c.Slug, &c.Label, &c.Type, &c.Status, &c.ArchivedAt,
				&c.ColumnName, &c.Currency, &optionsRaw, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.Version); err != nil {
				return fmt.Errorf("customfields: scan catalog row: %w", err)
			}
			if len(optionsRaw) > 0 {
				_ = json.Unmarshal(optionsRaw, &c.Options)
			}
			fields = append(fields, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("customfields: iterate catalog rows: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fields, nil
}
