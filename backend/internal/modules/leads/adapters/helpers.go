// Package adapters contains the leads module's PostgreSQL storage adapters.
package adapters

import (
	"context"
	"database/sql"
	"encoding/base64"
	"strings"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
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

// requireProvenance rejects an empty source or captured_by with a typed sentinel
// (data-model §1.6 provenance).
func requireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

// encodeKeysetCursor packs (sortVal, id) into one opaque, URL-safe token.
func encodeKeysetCursor(sortVal, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortVal + "\x00" + id))
}

// decodeKeysetCursor unpacks a token from encodeKeysetCursor. ok=false for an
// empty or malformed token, in which case the caller treats it as "first page".
func decodeKeysetCursor(cursor string) (sortVal, id string, ok bool) {
	if cursor == "" {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// nullStrParam binds s as a SQL value, or NULL when s is empty.
func nullStrParam(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullStr reads a *string from an updates map; nil when key is absent or not a string.
func nullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}
