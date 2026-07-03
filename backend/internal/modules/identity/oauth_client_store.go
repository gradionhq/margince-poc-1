package crmauth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO oauth_client (workspace_id, redirect_uris)
		VALUES ($1::uuid, $2::text[])
		RETURNING client_id`,
		workspaceID, arr).Scan(&rec.ClientID)
	if err != nil {
		return OAuthClientRecord{}, err
	}
	rec.WorkspaceID = workspaceID
	rec.RedirectURIs = redirectURIs
	return rec, nil
}

// Lookup returns a registered client by client_id.
func (s *OAuthClientStore) Lookup(ctx context.Context, clientID string) (OAuthClientRecord, error) {
	var rec OAuthClientRecord
	var raw []byte
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
