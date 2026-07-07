package crmauth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// PassportRecord is a loaded passport row.
type PassportRecord struct {
	ID          string
	WorkspaceID string
	GrantedBy   string
	Scopes      []string
	RevokedAt   *time.Time
}

// PassportStore manages passport rows.
type PassportStore struct{ db *sql.DB }

// NewPassportStore returns a PassportStore.
func NewPassportStore(db *sql.DB) *PassportStore { return &PassportStore{db: db} }

// Create mints a passport token, stores its hash, returns raw token + record.
func (s *PassportStore) Create(ctx context.Context, workspaceID, grantedBy string, scopes []string, expiresIn time.Duration) (rawToken string, rec PassportRecord, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", rec, err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256sum(rawToken)
	expiresAt := time.Now().UTC().Add(expiresIn)
	scopeArr := "{" + strings.Join(scopes, ",") + "}"
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO passport (workspace_id, granted_by, scopes, token_hash, expires_at)
			VALUES ($1::uuid, $2::uuid, $3::text[], $4, $5)
			RETURNING id`,
			workspaceID, grantedBy, scopeArr, hash, expiresAt).Scan(&rec.ID)
	})
	if err != nil {
		return "", rec, err
	}
	rec.WorkspaceID = workspaceID
	rec.GrantedBy = grantedBy
	rec.Scopes = scopes
	return rawToken, rec, nil
}

// Lookup returns a valid (not revoked, not expired) passport by raw token.
//
// rls-exempt: same chicken-and-egg shape as SessionStore.Lookup (GH-209
// escalation resolution, Option 1) — the workspace is unknown until this
// opaque passport token resolves it. The raw token's entropy is the security
// boundary pre-resolution.
func (s *PassportStore) Lookup(ctx context.Context, rawToken string) (PassportRecord, error) {
	hash := sha256sum(rawToken)
	var rec PassportRecord
	var scopesRaw []byte
	// rls-exempt: see doc comment above.
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, granted_by, scopes, revoked_at
		FROM passport
		WHERE token_hash=$1 AND expires_at > now() AND revoked_at IS NULL`,
		hash).Scan(&rec.ID, &rec.WorkspaceID, &rec.GrantedBy, &scopesRaw, &rec.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, ErrNotFound
	}
	if err != nil {
		return rec, err
	}
	// scopesRaw is postgres array literal: {scope1,scope2}
	rec.Scopes = parsePostgresTextArray(string(scopesRaw))
	return rec, nil
}

// Revoke sets revoked_at for a passport.
func (s *PassportStore) Revoke(ctx context.Context, id, workspaceID string) error {
	return database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE passport SET revoked_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND revoked_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func parsePostgresTextArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
