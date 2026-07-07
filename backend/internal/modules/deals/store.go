package deals

import (
	"context"
	"database/sql"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// withWorkspaceTx runs fn inside a single tx as the non-superuser margince_app role
// with app.workspace_id set, so FORCE RLS is actually enforced on every CRUD query
// (data-model §1.3). Delegates to the shared platform/database seam (GH-209 WS-A) —
// kept as a same-package unexported wrapper (not re-exported at every call site) so
// none of this file's existing withWorkspaceTx(...) callers need to change.
func withWorkspaceTx(ctx context.Context, db *sql.DB, workspaceID string, fn func(tx *sql.Tx) error) error {
	return database.WithWorkspaceTx(ctx, db, workspaceID, fn)
}

func nullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

func nullInt(m map[string]any, key string) *int64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			i := int64(n)
			return &i
		case int64:
			return &n
		}
	}
	return nil
}

// nullBool reads a *bool bounded-update field the same way nullStr/nullInt
// read their types — the pipeline store's is_default is the first bool-typed
// bounded field, so this is new (not a duplicate of anything in directory).
func nullBool(m map[string]any, key string) *bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return &b
		}
	}
	return nil
}
