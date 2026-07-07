package crmcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// ErrGrantExceedsGrantorAccess is returned by RecordGrantStore.Create when the
// granting principal is attempting to grant an access level (or to a record)
// they do not themselves hold — a grant can never exceed the granting
// principal's own access (data-model DM-DDL-5, contract crm.yaml
// createRecordGrant description).
var ErrGrantExceedsGrantorAccess = errors.New("crmcore: grant exceeds granting principal's own access")

// RecordGrant mirrors the contract's RecordGrant schema (crm.yaml).
type RecordGrant struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"-"`
	RecordType  string     `json:"record_type"`
	RecordID    string     `json:"record_id"`
	SubjectType string     `json:"subject_type"`
	SubjectID   string     `json:"subject_id"`
	Access      string     `json:"access"`
	GrantedBy   string     `json:"granted_by"`
	Reason      *string    `json:"reason"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Version     int64      `json:"version"`
}

// CreateRecordGrantInput is the store-level create/upsert request.
type CreateRecordGrantInput struct {
	WorkspaceID string
	RecordType  string
	RecordID    string
	SubjectType string
	SubjectID   string
	Access      string
	GrantedBy   string
	Reason      *string
	ExpiresAt   *time.Time
	// GrantorOwnAccess is the granting principal's own effective access to
	// the record ("read"|"write"|""), resolved by the caller (transport
	// layer) from whatever scope-check primitive is available before calling
	// Create — Create itself only enforces the ordering, it doesn't resolve
	// scope (crmauth has no existing per-record scope resolver to call into;
	// see plan design note below).
	GrantorOwnAccess string
}

// RecordGrantStore implements listRecordGrants/createRecordGrant/revokeRecordGrant
// (crm.yaml) against the record_grant table (DM-DDL-5, migration 000069).
type RecordGrantStore struct{ db *sql.DB }

// NewRecordGrantStore constructs a RecordGrantStore.
func NewRecordGrantStore(db *sql.DB) *RecordGrantStore { return &RecordGrantStore{db: db} }

// accessRank orders access levels so "write" satisfies "read" (contract:
// "'write' also satisfies 'read'").
var accessRank = map[string]int{"read": 1, "write": 2}

// Create upserts a grant, idempotent on (record_type, record_id, subject_type,
// subject_id) — a second call updates access/expires_at/reason on the
// existing row rather than duplicating it (record_grant_unique, migration
// 000069). Rejects a grant whose access exceeds the granting principal's own
// access to the record. Writes one audit_log row (action=record_share) in the
// same tx as the upsert.
func (s *RecordGrantStore) Create(ctx context.Context, in CreateRecordGrantInput) (RecordGrant, error) {
	if accessRank[in.Access] > accessRank[in.GrantorOwnAccess] {
		return RecordGrant{}, ErrGrantExceedsGrantorAccess
	}

	var g RecordGrant
	err := database.WithWorkspaceTx(ctx, s.db, in.WorkspaceID, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(
			ctx, `
			INSERT INTO record_grant (workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by, reason, expires_at)
			VALUES ($1::uuid,$2,$3::uuid,$4,$5::uuid,$6,$7::uuid,$8,$9)
			ON CONFLICT (workspace_id, record_type, record_id, subject_type, subject_id)
			DO UPDATE SET access=EXCLUDED.access, reason=EXCLUDED.reason, expires_at=EXCLUDED.expires_at,
			              granted_by=EXCLUDED.granted_by, version=record_grant.version+1
			RETURNING id, workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by, reason, expires_at, created_at, version`,
			in.WorkspaceID, in.RecordType, in.RecordID, in.SubjectType, in.SubjectID, in.Access, in.GrantedBy, in.Reason, in.ExpiresAt,
		).Scan(&g.ID, &g.WorkspaceID, &g.RecordType, &g.RecordID, &g.SubjectType, &g.SubjectID, &g.Access, &g.GrantedBy, &g.Reason, &g.ExpiresAt, &g.CreatedAt, &g.Version)
		if err != nil {
			return fmt.Errorf("record grant upsert: %w", err)
		}

		e := crmaudit.EntryFromPrincipal(ctx, "record_share", "record_grant", &g.ID, nil, map[string]any{
			"record_type": in.RecordType, "record_id": in.RecordID,
			"subject_type": in.SubjectType, "subject_id": in.SubjectID, "access": in.Access,
		})
		e.WorkspaceID = in.WorkspaceID
		_, err = crmaudit.WriteTx(ctx, tx, e)
		return err
	})
	if err != nil {
		return RecordGrant{}, err
	}
	return g, nil
}

// Revoke deletes the grant and writes one audit_log row (action=record_unshare)
// in the same tx.
func (s *RecordGrantStore) Revoke(ctx context.Context, id, workspaceID string) error {
	return database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var recordType, recordID string
		err := tx.QueryRowContext(ctx,
			`DELETE FROM record_grant WHERE id=$1::uuid AND workspace_id=$2::uuid RETURNING record_type, record_id`,
			id, workspaceID).Scan(&recordType, &recordID)
		if err != nil {
			return err
		}
		e := crmaudit.EntryFromPrincipal(ctx, "record_unshare", "record_grant", &id, nil, map[string]any{
			"record_type": recordType, "record_id": recordID,
		})
		e.WorkspaceID = workspaceID
		_, err = crmaudit.WriteTx(ctx, tx, e)
		return err
	})
}

// RecordGrantListFilter holds optional predicates for List (all optional —
// the contract's listRecordGrants supports filtering by record OR by
// subject). Zero value = no extra filters.
type RecordGrantListFilter struct {
	RecordType  string
	RecordID    string
	SubjectType string
	SubjectID   string
	Cursor      string
}

// buildRecordGrantListWhere builds the dynamic filter fragment of List's WHERE
// clause from filter's optional predicates. $1 and $2 are reserved by the
// caller for workspace_id and cursor, so numbering starts at 3.
func buildRecordGrantListWhere(filter RecordGrantListFilter) (whereClause string, args []any, nextArgIdx int) {
	n := 2
	if filter.RecordType != "" {
		n++
		whereClause += fmt.Sprintf(` AND record_type=$%d`, n)
		args = append(args, filter.RecordType)
	}
	if filter.RecordID != "" {
		n++
		whereClause += fmt.Sprintf(` AND record_id=$%d::uuid`, n)
		args = append(args, filter.RecordID)
	}
	if filter.SubjectType != "" {
		n++
		whereClause += fmt.Sprintf(` AND subject_type=$%d`, n)
		args = append(args, filter.SubjectType)
	}
	if filter.SubjectID != "" {
		n++
		whereClause += fmt.Sprintf(` AND subject_id=$%d::uuid`, n)
		args = append(args, filter.SubjectID)
	}
	return whereClause, args, n
}

// List returns grants matching the given filter (all optional — the
// contract's listRecordGrants supports filtering by record OR by subject).
func (s *RecordGrantStore) List(ctx context.Context, workspaceID string, filter RecordGrantListFilter, limit int) ([]RecordGrant, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	extraWhere, extraArgs, n := buildRecordGrantListWhere(filter)
	out := []RecordGrant{}
	var nextCursor string
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		where := `workspace_id=$1::uuid AND ($2 = '' OR id::text > $2)` + extraWhere
		args := append([]any{workspaceID, filter.Cursor}, extraArgs...)
		n++
		args = append(args, limit+1)
		//nolint:gosec // G202: `where` injects only bound-param indices ($N), never user input; all filter values are passed via args
		rows, err := tx.QueryContext(ctx,
			`SELECT id, workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by, reason, expires_at, created_at, version
			FROM record_grant WHERE `+where+` ORDER BY id LIMIT $`+strconv.Itoa(n),
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var g RecordGrant
			if err := rows.Scan(&g.ID, &g.WorkspaceID, &g.RecordType, &g.RecordID, &g.SubjectType, &g.SubjectID, &g.Access, &g.GrantedBy, &g.Reason, &g.ExpiresAt, &g.CreatedAt, &g.Version); err != nil {
				return err
			}
			out = append(out, g)
		}
		if len(out) > limit {
			nextCursor = out[limit-1].ID
			out = out[:limit]
		}
		return rows.Err()
	})
	return out, nextCursor, err
}
