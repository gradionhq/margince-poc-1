package crmauth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// OAuthClientRecord is a registered DCR (RFC 7591) public client.
type OAuthClientRecord struct {
	ClientID     string
	WorkspaceID  string
	RedirectURIs []string
}

// OAuthClientStore manages oauth_client rows — public clients only (PKCE
// replaces the client secret per OAuth 2.1).
type OAuthClientStore struct{ db *sql.DB }

// NewOAuthClientStore returns an OAuthClientStore.
func NewOAuthClientStore(db *sql.DB) *OAuthClientStore { return &OAuthClientStore{db: db} }

// Register inserts a new public client and returns its generated client_id.
func (s *OAuthClientStore) Register(ctx context.Context, workspaceID string, redirectURIs []string) (OAuthClientRecord, error) {
	var rec OAuthClientRecord
	arr := "{" + strings.Join(redirectURIs, ",") + "}"
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			INSERT INTO oauth_client (workspace_id, redirect_uris)
			VALUES ($1::uuid, $2::text[])
			RETURNING client_id`,
			workspaceID, arr).Scan(&rec.ClientID)
	})
	if err != nil {
		return OAuthClientRecord{}, err
	}
	rec.WorkspaceID = workspaceID
	rec.RedirectURIs = redirectURIs
	return rec, nil
}

// Lookup returns a registered client by client_id.
//
// rls-exempt: client_id resolves workspace_id, the same chicken-and-egg shape
// as SessionStore.Lookup/PassportStore.Lookup (GH-209 escalation resolution,
// Option 1) — there is no workspace to scope by before this query resolves
// one. client_id is a uuid primary key, not attacker-guessable in practice,
// and DCR client registration (RFC 7591) is itself the trust boundary here.
func (s *OAuthClientStore) Lookup(ctx context.Context, clientID string) (OAuthClientRecord, error) {
	var rec OAuthClientRecord
	var raw []byte
	// rls-exempt: see doc comment above.
	err := s.db.QueryRowContext(ctx, `
		SELECT client_id, workspace_id, redirect_uris
		FROM oauth_client
		WHERE client_id=$1::uuid`,
		clientID).Scan(&rec.ClientID, &rec.WorkspaceID, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, ErrNotFound
	}
	if err != nil {
		return rec, err
	}
	rec.RedirectURIs = parsePostgresTextArray(string(raw))
	return rec, nil
}
