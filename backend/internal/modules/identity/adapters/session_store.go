package adapters

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

const (
	sessionDuration     = 24 * time.Hour
	sessionIdleDuration = 2 * time.Hour
	cookieName          = "crm_session"
)

// CookieName is the session cookie name.
const CookieName = cookieName

// ErrNotFound mirrors errs.ErrNotFound but is defined locally because the module
// DAG (.go-arch-lint.yml) only permits crm-auth to depend on crmctx, not errs.
var ErrNotFound = errors.New("not found")

// SessionRecord is a loaded session row.
type SessionRecord struct {
	ID          string
	WorkspaceID string
	UserID      string
	UserAgent   *string
	IP          *string
	RevokedAt   *time.Time
}

// SessionStore manages session rows.
type SessionStore struct{ db *sql.DB }

// NewSessionStore returns a SessionStore.
func NewSessionStore(db *sql.DB) *SessionStore { return &SessionStore{db: db} }

// Create mints a new raw token, stores its SHA-256 hash, returns the raw token.
func (s *SessionStore) Create(ctx context.Context, workspaceID, userID, userAgent, ip string) (rawToken string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256sum(rawToken)
	now := time.Now().UTC()
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO session (workspace_id, user_id, token_hash, expires_at, idle_expires_at, last_seen_at, user_agent, ip)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::inet)`,
			workspaceID, userID, hash,
			now.Add(sessionDuration), now.Add(sessionIdleDuration), now,
			nullableStr(userAgent), nullableStr(ip))
		return err
	})
	return rawToken, err
}

// Lookup returns the session for rawToken if valid (not expired / idle-expired).
//
// rls-exempt: the workspace is unknown until this opaque session token
// resolves it (GH-209 escalation resolution, Option 1) — cannot scope by
// workspace_id before the query runs. The raw token's 256-bit entropy
// (crypto/rand) is the security boundary here, the same trust model an
// opaque session/OAuth-code lookup commonly uses. Every OTHER method here,
// which already knows workspace_id up front, is scoped through
// platform/database (see Create/Touch/Delete/Revoke).
func (s *SessionStore) Lookup(ctx context.Context, rawToken string) (SessionRecord, error) {
	hash := sha256sum(rawToken)
	var rec SessionRecord
	// rls-exempt: see doc comment above.
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, user_id, user_agent, host(ip), revoked_at FROM session
		WHERE token_hash=$1 AND expires_at > now() AND idle_expires_at > now() AND revoked_at IS NULL`,
		hash).Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.UserAgent, &rec.IP, &rec.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, ErrNotFound
	}
	return rec, err
}

// Touch updates idle_expires_at for the session. workspaceID is required (not
// derived) so the update is RLS-scoped — the caller (SessionMiddleware) already
// has it from the Lookup that preceded this call.
func (s *SessionStore) Touch(ctx context.Context, workspaceID, sessionID string) {
	now := time.Now().UTC()
	_ = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE session SET idle_expires_at=$1, last_seen_at=$2 WHERE id=$3::uuid`,
			now.Add(sessionIdleDuration), now, sessionID)
		return err
	})
}

// Delete removes a session by ID. workspaceID is required (not derived) so the
// delete is RLS-scoped — the caller already has it from the Lookup that
// preceded this call.
func (s *SessionStore) Delete(ctx context.Context, workspaceID, sessionID string) error {
	return database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM session WHERE id=$1::uuid`, sessionID)
		return err
	})
}

// Revoke soft-revokes a session. workspaceID is required so the update is
// RLS-scoped through platform/database, same as Touch/Delete.
func (s *SessionStore) Revoke(ctx context.Context, sessionID, workspaceID string) error {
	return database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE session SET revoked_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND revoked_at IS NULL`,
			sessionID, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func sha256sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// nullableStr returns nil for an empty string (maps to SQL NULL).
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SHA256SumExported is exported for integration test use only.
func SHA256SumExported(s string) string { return sha256sum(s) }
