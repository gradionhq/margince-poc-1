package crmauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const (
	sessionDuration     = 24 * time.Hour
	sessionIdleDuration = 2 * time.Hour
	cookieName          = "crm_session"
)

// ErrNotFound mirrors errs.ErrNotFound but is defined locally because the module
// DAG (.go-arch-lint.yml) only permits crm-auth to depend on crmctx, not errs.
// Callers compose it into HTTP 404 / 401 responses at the composition root.
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
// userAgent/ip are the login request's provenance (session.user_agent/ip,
// DM-DDL-6) — an empty string stores NULL.
func (s *SessionStore) Create(ctx context.Context, workspaceID, userID, userAgent, ip string) (rawToken string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256sum(rawToken)
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO session (workspace_id, user_id, token_hash, expires_at, idle_expires_at, last_seen_at, user_agent, ip)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::inet)`,
		workspaceID, userID, hash,
		now.Add(sessionDuration), now.Add(sessionIdleDuration), now,
		nullableStr(userAgent), nullableStr(ip))
	return rawToken, err
}

// Lookup returns the session for rawToken if valid (not expired, not
// idle-expired, not revoked — a revoked session must stop authenticating).
func (s *SessionStore) Lookup(ctx context.Context, rawToken string) (SessionRecord, error) {
	hash := sha256sum(rawToken)
	var rec SessionRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, user_id, user_agent, host(ip), revoked_at FROM session
		WHERE token_hash=$1 AND expires_at > now() AND idle_expires_at > now() AND revoked_at IS NULL`,
		hash).Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.UserAgent, &rec.IP, &rec.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, ErrNotFound
	}
	return rec, err
}

// Touch updates idle_expires_at for the session.
func (s *SessionStore) Touch(ctx context.Context, sessionID string) {
	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx, `
		UPDATE session SET idle_expires_at=$1, last_seen_at=$2 WHERE id=$3::uuid`,
		now.Add(sessionIdleDuration), now, sessionID)
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE id=$1::uuid`, sessionID)
	return err
}

// Revoke soft-revokes a session (mirrors PassportStore.Revoke exactly). A
// revoked session's Lookup fails with ErrNotFound instead of hard-deleting the
// row, so the restored revoked_at column (D4) has a real writer.
func (s *SessionStore) Revoke(ctx context.Context, sessionID, workspaceID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE session SET revoked_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND revoked_at IS NULL`,
		sessionID, workspaceID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
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

// SHA256SumExported is exported for integration test use only: it lets tests mint
// the same token hash the SessionStore stores, so they can insert fixture rows
// (e.g. an already-expired session) that Lookup will resolve.
func SHA256SumExported(s string) string { return sha256sum(s) }
