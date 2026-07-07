package crmauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// IncumbentConnectionRecord is one loaded incumbent_connection row.
type IncumbentConnectionRecord struct {
	ID          string
	WorkspaceID string
	Connector   string
	Status      string
	Scopes      []string
	ConnectedAt time.Time
	RevokedAt   *time.Time
}

// DBExec is satisfied by both *sql.Tx and *sql.DB — the same shape as
// crmaudit.DBExec, duplicated here so crm-auth's stores don't import crmaudit
// just for this interface (crm-auth mayDependOn does not include audit).
type DBExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// IncumbentConnectionStore manages incumbent_connection rows (one per
// workspace+connector — a workspace has at most one live HubSpot connection).
//
// Backed by DBExec (satisfied by both *sql.DB and *sql.Tx, mirroring
// crmaudit.DBExec) rather than a concrete *sql.DB, so a caller that needs a
// connection mutation to commit atomically with an audit row (P12 — see
// cmd/server/hubspot_handler.go's callback/rotate/revoke handlers) can pass a
// *sql.Tx to the same constructor instead of a separate Tx-suffixed type.
type IncumbentConnectionStore struct{ db DBExec }

// NewIncumbentConnectionStore returns an IncumbentConnectionStore backed by
// db, which may be a *sql.DB (plain reads/writes) or a *sql.Tx (when the
// caller needs this store's mutation to share a transaction with e.g. an
// audit-log write).
func NewIncumbentConnectionStore(db DBExec) *IncumbentConnectionStore {
	return &IncumbentConnectionStore{db: db}
}

// Create inserts a new active connection for workspaceID+connector with scopes.
func (s *IncumbentConnectionStore) Create(ctx context.Context, workspaceID, connector string, scopes []string) (IncumbentConnectionRecord, error) {
	var rec IncumbentConnectionRecord
	scopeArr := "{" + strings.Join(scopes, ",") + "}"
	if err := database.SetWorkspaceScope(ctx, s.db, workspaceID); err != nil {
		return rec, fmt.Errorf("incumbent connection: guc: %w", err)
	}
	// rls-exempt: SetWorkspaceScope already called above on the caller-supplied DBExec (D1 shape — caller owns tx lifecycle).
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO incumbent_connection (workspace_id, connector, scopes)
		VALUES ($1::uuid, $2, $3::text[])
		RETURNING id, connected_at`,
		workspaceID, connector, scopeArr).Scan(&rec.ID, &rec.ConnectedAt)
	if err != nil {
		return rec, err
	}
	rec.WorkspaceID = workspaceID
	rec.Connector = connector
	rec.Status = "active"
	rec.Scopes = scopes
	return rec, nil
}

// Get returns the live connection row for workspaceID+connector.
func (s *IncumbentConnectionStore) Get(ctx context.Context, workspaceID, connector string) (IncumbentConnectionRecord, error) {
	var rec IncumbentConnectionRecord
	var scopesRaw []byte
	if err := database.SetWorkspaceScope(ctx, s.db, workspaceID); err != nil {
		return rec, fmt.Errorf("incumbent connection: guc: %w", err)
	}
	// rls-exempt: SetWorkspaceScope already called above on the caller-supplied DBExec (D1 shape — caller owns tx lifecycle).
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, connector, status, scopes, connected_at, revoked_at
		FROM incumbent_connection
		WHERE workspace_id=$1::uuid AND connector=$2`,
		workspaceID, connector).Scan(&rec.ID, &rec.WorkspaceID, &rec.Connector, &rec.Status, &scopesRaw, &rec.ConnectedAt, &rec.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return rec, ErrNotFound
	}
	if err != nil {
		return rec, err
	}
	rec.Scopes = parsePostgresTextArray(string(scopesRaw))
	return rec, nil
}

// Revoke sets status='revoked', revoked_at=now() for an active connection.
// Fail-closed: a second Revoke on an already-revoked (or absent) connection
// returns ErrNotFound rather than silently no-oping, mirroring
// PassportStore.Revoke's revoked_at IS NULL fail-closed idiom.
func (s *IncumbentConnectionStore) Revoke(ctx context.Context, id, workspaceID string) error {
	if err := database.SetWorkspaceScope(ctx, s.db, workspaceID); err != nil {
		return fmt.Errorf("incumbent connection: guc: %w", err)
	}
	// rls-exempt: SetWorkspaceScope already called above on the caller-supplied DBExec (D1 shape — caller owns tx lifecycle).
	res, err := s.db.ExecContext(ctx, `
		UPDATE incumbent_connection SET status='revoked', revoked_at=now()
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND status='active'`,
		id, workspaceID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
