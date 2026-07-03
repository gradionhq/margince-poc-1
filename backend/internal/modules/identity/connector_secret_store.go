package crmauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

// ConnectorSecretRecord is one loaded connector_secret row (ciphertext only —
// callers get plaintext solely through Lookup, never a raw field read).
type ConnectorSecretRecord struct {
	ID           string
	WorkspaceID  string
	ConnectionID string
	Ciphertext   []byte
	KMSKeyID     string
	RotatedAt    time.Time
}

// ConnectorSecretStore seals/unseals connector token material through a
// keyvault.KeyProvider and persists ciphertext-only rows.
//
// Rotate/Put both APPEND a new connector_secret row rather than updating one in
// place (the "store-level concern" the spec leaves to the implementer): a
// superseded row stays queryable for audit/incident review, and Lookup always
// resolves the latest by rotated_at DESC. This trades a small amount of extra
// storage for never losing the ability to inspect what a prior token rotation
// looked like (kms_key_id, timing) after the fact.
//
// Backed by DBExec (see incumbent_connection_store.go) so a caller can pass a
// live *sql.Tx when this store's write must commit atomically with an audit
// row (P12).
type ConnectorSecretStore struct {
	db       DBExec
	provider keyvault.KeyProvider
}

// NewConnectorSecretStore returns a ConnectorSecretStore backed by db (a
// *sql.DB or a *sql.Tx) and provider.
func NewConnectorSecretStore(db DBExec, provider keyvault.KeyProvider) *ConnectorSecretStore {
	return &ConnectorSecretStore{db: db, provider: provider}
}

// requireActiveConnection returns an error if the connection is absent or not
// 'active'. Shared by Rotate and Lookup so both fail closed identically on a
// revoked connection.
func (s *ConnectorSecretStore) requireActiveConnection(ctx context.Context, workspaceID, connectionID string) error {
	var status string
	if err := s.db.QueryRowContext(ctx, `
		SELECT status FROM incumbent_connection WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		connectionID, workspaceID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "active" {
		return fmt.Errorf("connector secret: connection %s is %s, not active", connectionID, status)
	}
	return nil
}

// Put seals plaintext and inserts a new connector_secret row for connectionID.
// Put does not itself require the connection to be active — it is also the
// very first token write at connect time, before the connection can have any
// other status — but every OTHER caller of this store (Rotate, Lookup) does
// guard on active status; a revoked connection's Rotate/Lookup both fail
// closed rather than accepting a still-plausible-looking write or read.
func (s *ConnectorSecretStore) Put(ctx context.Context, workspaceID, connectionID string, plaintext []byte) (ConnectorSecretRecord, error) {
	var rec ConnectorSecretRecord
	ciphertext, kmsKeyID, err := s.provider.Seal(ctx, plaintext)
	if err != nil {
		return rec, fmt.Errorf("connector secret: seal: %w", err)
	}
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO connector_secret (workspace_id, connection_id, ciphertext, kms_key_id)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING id, rotated_at`,
		workspaceID, connectionID, ciphertext, kmsKeyID).Scan(&rec.ID, &rec.RotatedAt)
	if err != nil {
		return rec, err
	}
	rec.WorkspaceID = workspaceID
	rec.ConnectionID = connectionID
	rec.Ciphertext = ciphertext
	rec.KMSKeyID = kmsKeyID
	return rec, nil
}

// Rotate appends a new connector_secret row for an ACTIVE connection only —
// guarded the same way Lookup is guarded (requireActiveConnection), so a
// rotate against a revoked connection fails closed instead of silently
// appending a dead write nobody can ever Lookup back out.
func (s *ConnectorSecretStore) Rotate(ctx context.Context, workspaceID, connectionID string, newPlaintext []byte) (ConnectorSecretRecord, error) {
	if err := s.requireActiveConnection(ctx, workspaceID, connectionID); err != nil {
		return ConnectorSecretRecord{}, err
	}
	return s.Put(ctx, workspaceID, connectionID, newPlaintext)
}

// Lookup resolves the latest connector_secret row for connectionID and returns
// its unsealed plaintext. Fails closed if the parent incumbent_connection is
// not 'active' (revoked) — the caller never gets a stale token back silently.
func (s *ConnectorSecretStore) Lookup(ctx context.Context, workspaceID, connectionID string) ([]byte, error) {
	if err := s.requireActiveConnection(ctx, workspaceID, connectionID); err != nil {
		return nil, err
	}

	var ciphertext []byte
	var kmsKeyID string
	err := s.db.QueryRowContext(ctx, `
		SELECT ciphertext, kms_key_id FROM connector_secret
		WHERE workspace_id=$1::uuid AND connection_id=$2::uuid
		ORDER BY rotated_at DESC LIMIT 1`,
		workspaceID, connectionID).Scan(&ciphertext, &kmsKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	plaintext, err := s.provider.Unseal(ctx, ciphertext, kmsKeyID)
	if err != nil {
		return nil, fmt.Errorf("connector secret: unseal: %w", err)
	}
	return plaintext, nil
}
