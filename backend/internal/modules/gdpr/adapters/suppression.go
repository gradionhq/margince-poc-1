package adapters

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
)

// Querier is satisfied by *sql.DB, *sql.Tx, and *sql.Conn.
type Querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// Hash returns the SHA-256 hex digest of the email, lowercased and trimmed.
// This is the canonical key stored in erasure_suppression — never the plaintext email.
func Hash(email string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return fmt.Sprintf("%x", h)
}

// IsSuppressed reports whether email is in erasure_suppression for wsID.
// q must be a *sql.DB or *sql.Tx with app.workspace_id GUC already set.
func IsSuppressed(ctx context.Context, q Querier, wsID, email string) (bool, error) {
	var n int
	err := q.QueryRowContext(
		ctx,
		`SELECT count(*) FROM erasure_suppression WHERE workspace_id=$1::uuid AND email_hash=$2`,
		wsID, Hash(email),
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("suppression.IsSuppressed: %w", err)
	}
	return n > 0, nil
}

// suppressionAdd inserts email's hash into erasure_suppression within the caller's transaction.
// Silently ignores conflicts (idempotent).
func suppressionAdd(ctx context.Context, tx *sql.Tx, wsID, email string) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO erasure_suppression (workspace_id, email_hash)
         VALUES ($1::uuid, $2)
         ON CONFLICT (workspace_id, email_hash) DO NOTHING`,
		wsID, Hash(email),
	)
	if err != nil {
		return fmt.Errorf("suppression.Add %s: %w", wsID, err)
	}
	return nil
}
