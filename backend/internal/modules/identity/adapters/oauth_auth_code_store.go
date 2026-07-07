package adapters

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

// ErrInvalidGrant is returned by AuthCodeStore.Consume on any failure that
// must fail closed.
var ErrInvalidGrant = errors.New("invalid grant")

// AuthCodeRecord is the resolved grant behind a consumed authorization code.
type AuthCodeRecord struct {
	ClientID    string
	WorkspaceID string
	RedirectURI string
	Scopes      []string
	GrantedBy   string
}

// AuthCodeStore manages one-time-use, short-lived PKCE authorization codes.
type AuthCodeStore struct{ db *sql.DB }

// NewAuthCodeStore returns an AuthCodeStore.
func NewAuthCodeStore(db *sql.DB) *AuthCodeStore { return &AuthCodeStore{db: db} }

// PKCEChallengeS256 derives the RFC 7636 S256 code_challenge from a code_verifier.
func PKCEChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Issue mints a one-time authorization code.
func (s *AuthCodeStore) Issue(ctx context.Context, clientID, workspaceID, codeChallenge, redirectURI string, scopes []string, grantedBy string, ttl time.Duration) (rawCode string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", err
	}
	rawCode = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256sum(rawCode)
	expiresAt := time.Now().UTC().Add(ttl)
	scopeArr := "{" + strings.Join(scopes, ",") + "}"
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO oauth_auth_code
			(code_hash, client_id, workspace_id, code_challenge, redirect_uri, scopes, granted_by, expires_at)
		VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6::text[], $7::uuid, $8)`,
		hash, clientID, workspaceID, codeChallenge, redirectURI, scopeArr, grantedBy, expiresAt)
	if err != nil {
		return "", err
	}
	return rawCode, nil
}

// Consume atomically validates and marks a code used in one transaction.
func (s *AuthCodeStore) Consume(ctx context.Context, rawCode, codeVerifier string) (AuthCodeRecord, error) {
	hash := sha256sum(rawCode)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthCodeRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var rec AuthCodeRecord
	var codeChallenge string
	var scopesRaw []byte
	err = tx.QueryRowContext(ctx, `
		SELECT client_id, workspace_id, code_challenge, redirect_uri, scopes, granted_by
		FROM oauth_auth_code
		WHERE code_hash=$1 AND expires_at > now() AND used_at IS NULL
		FOR UPDATE`,
		hash).Scan(&rec.ClientID, &rec.WorkspaceID, &codeChallenge, &rec.RedirectURI, &scopesRaw, &rec.GrantedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthCodeRecord{}, ErrInvalidGrant
	}
	if err != nil {
		return AuthCodeRecord{}, err
	}

	computed := PKCEChallengeS256(codeVerifier)
	if subtle.ConstantTimeCompare([]byte(computed), []byte(codeChallenge)) != 1 {
		return AuthCodeRecord{}, ErrInvalidGrant
	}

	if _, err := tx.ExecContext(ctx, `UPDATE oauth_auth_code SET used_at=now() WHERE code_hash=$1`, hash); err != nil {
		return AuthCodeRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return AuthCodeRecord{}, err
	}

	rec.Scopes = parsePostgresTextArray(string(scopesRaw))
	return rec, nil
}
