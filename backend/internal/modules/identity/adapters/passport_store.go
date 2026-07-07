package adapters

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

// PassportRecord is a loaded passport row.
type PassportRecord struct {
	ID          string
	WorkspaceID string
	GrantedBy   string
	OnBehalfOf  *string
	Label       *string
	Scopes      []string
	RevokedAt   *time.Time
}

// PassportStore manages passport rows.
type PassportStore struct{ db *sql.DB }

// NewPassportStore returns a PassportStore.
func NewPassportStore(db *sql.DB) *PassportStore { return &PassportStore{db: db} }

// Create mints a passport token, stores its hash, returns raw token + record.
func (s *PassportStore) Create(ctx context.Context, workspaceID, grantedBy, onBehalfOf, label string, scopes []string, expiresIn time.Duration) (rawToken string, rec PassportRecord, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", rec, err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256sum(rawToken)
	expiresAt := time.Now().UTC().Add(expiresIn)
	scopeArr := "{" + strings.Join(scopes, ",") + "}"
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO passport (workspace_id, granted_by, on_behalf_of, label, scopes, token_hash, expires_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::text[], $6, $7)
		RETURNING id`,
		workspaceID, grantedBy, onBehalfOf, nullableStr(label), scopeArr, hash, expiresAt).Scan(&rec.ID)
	if err != nil {
		return "", rec, err
	}
	rec.WorkspaceID = workspaceID
	rec.GrantedBy = grantedBy
	rec.OnBehalfOf = nullableStr(onBehalfOf)
	rec.Label = nullableStr(label)
	rec.Scopes = scopes
	return rawToken, rec, nil
}

// Lookup returns a valid passport by raw token.
func (s *PassportStore) Lookup(ctx context.Context, rawToken string) (PassportRecord, error) {
	hash := sha256sum(rawToken)
	var rec PassportRecord
	var scopesRaw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, granted_by, on_behalf_of, label, scopes, revoked_at
		FROM passport
		WHERE token_hash=$1 AND expires_at > now() AND revoked_at IS NULL`,
		hash).Scan(&rec.ID, &rec.WorkspaceID, &rec.GrantedBy, &rec.OnBehalfOf, &rec.Label, &scopesRaw, &rec.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, ErrNotFound
	}
	if err != nil {
		return rec, err
	}
	rec.Scopes = parsePostgresTextArray(string(scopesRaw))
	return rec, nil
}

// Revoke sets revoked_at for a passport.
func (s *PassportStore) Revoke(ctx context.Context, id, workspaceID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE passport SET revoked_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND revoked_at IS NULL`,
		id, workspaceID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func parsePostgresTextArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
