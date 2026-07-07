package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

// ConnectorSecretRecord is one loaded connector_secret row.
type ConnectorSecretRecord struct {
	ID           string
	WorkspaceID  string
	ConnectionID string
	Ciphertext   []byte
	KMSKeyID     string
	RotatedAt    time.Time
}

// ConnectorSecretStore seals/unseals connector token material.
type ConnectorSecretStore struct {
	db       DBExec
	provider keyvault.KeyProvider
}

// NewConnectorSecretStore returns a ConnectorSecretStore.
func NewConnectorSecretStore(db DBExec, provider keyvault.KeyProvider) *ConnectorSecretStore {
	return &ConnectorSecretStore{db: db, provider: provider}
}

// requireActiveConnection returns an error if the connection is absent or not 'active'.
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

// Rotate appends a new connector_secret row for an ACTIVE connection only.
func (s *ConnectorSecretStore) Rotate(ctx context.Context, workspaceID, connectionID string, newPlaintext []byte) (ConnectorSecretRecord, error) {
	if err := s.requireActiveConnection(ctx, workspaceID, connectionID); err != nil {
		return ConnectorSecretRecord{}, err
	}
	return s.Put(ctx, workspaceID, connectionID, newPlaintext)
}

// Lookup resolves the latest connector_secret row for connectionID and returns its unsealed plaintext.
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
