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
}

// SessionStore manages session rows.
type SessionStore struct{ db *sql.DB }

// NewSessionStore returns a SessionStore.
func NewSessionStore(db *sql.DB) *SessionStore { return &SessionStore{db: db} }

// Create mints a new raw token, stores its SHA-256 hash, returns the raw token.
func (s *SessionStore) Create(ctx context.Context, workspaceID, userID string) (rawToken string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256sum(rawToken)
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO session (workspace_id, user_id, token_hash, expires_at, idle_expires_at, last_seen_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`,
		workspaceID, userID, hash,
		now.Add(sessionDuration), now.Add(sessionIdleDuration), now)
	return rawToken, err
}

// Lookup returns the session for rawToken if valid (not expired / idle-expired).
func (s *SessionStore) Lookup(ctx context.Context, rawToken string) (SessionRecord, error) {
	hash := sha256sum(rawToken)
	var rec SessionRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, user_id FROM session
		WHERE token_hash=$1 AND expires_at > now() AND idle_expires_at > now()`,
		hash).Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID)
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

func sha256sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// SHA256SumExported is exported for integration test use only: it lets tests mint
// the same token hash the SessionStore stores, so they can insert fixture rows
// (e.g. an already-expired session) that Lookup will resolve.
func SHA256SumExported(s string) string { return sha256sum(s) }
