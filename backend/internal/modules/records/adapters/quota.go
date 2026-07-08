// Package adapters contains the records module's SQL-backed store implementations
// for quota (RD-DDL-2 — owner XOR team revenue target).
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

const entityTypeQuota = "quota"

// Quota is a per-owner or per-team revenue target for one period (RD-DDL-2). Carries no
// source/captured_by — the quota table has no provenance columns at all (deliberate, per the
// contract's own Quota schema description).
type Quota struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	OwnerID     *string    `json:"owner_id"`
	TeamID      *string    `json:"team_id"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	TargetMinor int64      `json:"target_minor"`
	Currency    string     `json:"currency"`
	Version     int64      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
}

// ErrOwnerXorTeamRequired is returned when a quota create or update would leave
// both owner_id and team_id set, or neither set (RD-DDL-2).
var ErrOwnerXorTeamRequired = errors.New("owner_xor_team_required")

// OwnerXorTeamValid is RD-DDL-2's CHECK, mirrored in Go so both createQuota and updateQuota (the
// merged-state case) can validate before ever reaching the database CHECK.
func OwnerXorTeamValid(ownerID, teamID *string) bool {
	return (ownerID != nil) != (teamID != nil)
}

// QuotaListFilter narrows a List call to quotas owned by a specific owner or team.
type QuotaListFilter struct{ OwnerID, TeamID string }

// QuotaStore executes parameterized SQL against the quota table.
type QuotaStore struct{ db *sql.DB }

// NewQuotaStore returns a QuotaStore backed by db.
func NewQuotaStore(db *sql.DB) *QuotaStore { return &QuotaStore{db: db} }

func hasKey(m map[string]any, key string) bool { _, ok := m[key]; return ok }

func nullInt64(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
	}
	return nil
}

func nullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

// mergedUUIDPtr computes the post-merge value for a nullable UUID patch field:
// absent from patch → keep current; present+nil → explicit clear; present+string → set.
func mergedUUIDPtr(updates map[string]any, key string, current *string) *string {
	if !hasKey(updates, key) {
		return current
	}
	v := updates[key]
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		return &s
	}
	return current
}

func scanQuota(row interface{ Scan(dest ...any) error }) (Quota, error) {
	var q Quota
	var ownerID, teamID sql.NullString
	err := row.Scan(
		&q.ID, &q.WorkspaceID, &ownerID, &teamID,
		&q.PeriodStart, &q.PeriodEnd, &q.TargetMinor, &q.Currency,
		&q.Version, &q.CreatedAt, &q.UpdatedAt, &q.ArchivedAt,
	)
	if err != nil {
		return Quota{}, err
	}
	q.OwnerID = nullStringPtr(ownerID)
	q.TeamID = nullStringPtr(teamID)
	return q, nil
}

// Create inserts a quota row and its create audit_log entry in one workspace-scoped tx.
// Rejects an invalid owner-XOR-team state before the INSERT.
func (s *QuotaStore) Create(ctx context.Context, q Quota) (Quota, error) {
	if !OwnerXorTeamValid(q.OwnerID, q.TeamID) {
		return Quota{}, ErrOwnerXorTeamRequired
	}
	err := database.WithWorkspaceTx(ctx, s.db, q.WorkspaceID, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(
			ctx, `
			INSERT INTO quota (workspace_id, owner_id, team_id, period_start, period_end, target_minor, currency)
			VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6,$7)
			RETURNING id`,
			q.WorkspaceID, q.OwnerID, q.TeamID, q.PeriodStart, q.PeriodEnd, q.TargetMinor, q.Currency,
		).Scan(&q.ID); err != nil {
			return fmt.Errorf("quota create: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeQuota, &q.ID, nil, q)
		e.WorkspaceID = q.WorkspaceID
		_, err := crmaudit.WriteTx(ctx, tx, e)
		return err
	})
	if err != nil {
		return Quota{}, err
	}
	return s.Get(ctx, q.ID, q.WorkspaceID)
}

// Get returns one live (non-archived) quota by id, workspace-scoped. Returns errs.ErrNotFound
// when absent or archived.
func (s *QuotaStore) Get(ctx context.Context, id, workspaceID string) (Quota, error) {
	var q Quota
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		// query is a single literal string (no concatenation/formatting) — mirrors the
		// records/adapters record_visibility.go precedent (SonarCloud go:S2077).
		row := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, owner_id, team_id,
			       period_start, period_end, target_minor, currency,
			       version, created_at, updated_at, archived_at
			FROM quota WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		var scanErr error
		q, scanErr = scanQuota(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Quota{}, errs.ErrNotFound
	}
	return q, err
}

// getAny is like Get but includes archived rows — used by Archive to return the full entity.
func (s *QuotaStore) getAny(ctx context.Context, id, workspaceID string) (Quota, error) {
	var q Quota
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, owner_id, team_id,
			       period_start, period_end, target_minor, currency,
			       version, created_at, updated_at, archived_at
			FROM quota WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID)
		var scanErr error
		q, scanErr = scanQuota(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Quota{}, errs.ErrNotFound
	}
	return q, err
}

// List returns a cursor-paginated slice of quotas. When filter.OwnerID or filter.TeamID is
// non-empty, only quotas matching that owner/team are returned. includeArchived toggles archived-row
// visibility.
//
// The query is a single static string; every optional filter is applied via an always-bound
// empty-string/boolean guard on the parameter (mirrors people/adapters/store_record_grant.go's
// List — no WHERE text built at runtime; SonarCloud go:S2077).
func (s *QuotaStore) List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool, filter QuotaListFilter) ([]Quota, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := []Quota{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID, cursor, includeArchived, filter.OwnerID, filter.TeamID, limit + 1}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, owner_id, team_id,
			       period_start, period_end, target_minor, currency,
			       version, created_at, updated_at, archived_at
			FROM quota
			WHERE workspace_id=$1::uuid
			  AND ($2 = '' OR id::text > $2)
			  AND ($3 OR archived_at IS NULL)
			  AND ($4 = '' OR owner_id=$4::uuid)
			  AND ($5 = '' OR team_id=$5::uuid)
			ORDER BY id LIMIT $6`, args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			q, scanErr := scanQuota(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, q)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// Update applies a merge-patch to a quota inside a tx, re-validating the merged owner-XOR-team
// state (not just the patch body). ifMatch=0 skips the version check.
func (s *QuotaStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (Quota, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var curOwner, curTeam sql.NullString
		if err := tx.QueryRowContext(ctx,
			`SELECT owner_id, team_id FROM quota WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(&curOwner, &curTeam); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.ErrNotFound
			}
			return err
		}
		mergedOwner := mergedUUIDPtr(updates, "owner_id", nullStringPtr(curOwner))
		mergedTeam := mergedUUIDPtr(updates, "team_id", nullStringPtr(curTeam))
		if !OwnerXorTeamValid(mergedOwner, mergedTeam) {
			return ErrOwnerXorTeamRequired
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE quota
			SET owner_id     = CASE WHEN $3 THEN $4::uuid ELSE owner_id END,
			    team_id      = CASE WHEN $5 THEN $6::uuid ELSE team_id END,
			    period_start = COALESCE($7, period_start),
			    period_end   = COALESCE($8, period_end),
			    target_minor = COALESCE($9, target_minor),
			    currency     = COALESCE($10, currency)
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
			  AND ($11 = 0 OR version = $11)`,
			id, workspaceID,
			hasKey(updates, "owner_id"), mergedOwner,
			hasKey(updates, "team_id"), mergedTeam,
			sqlutil.NullStr(updates, "period_start"),
			sqlutil.NullStr(updates, "period_end"),
			nullInt64(updates, "target_minor"),
			sqlutil.NullStr(updates, "currency"),
			ifMatch)
		if err != nil {
			return fmt.Errorf("quota update: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if ifMatch != 0 {
				return errs.ErrVersionSkew
			}
			return errs.ErrNotFound
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeQuota, &id, nil, nil)
		e.WorkspaceID = workspaceID
		_, err = crmaudit.WriteTx(ctx, tx, e)
		return err
	})
	if err != nil {
		return Quota{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes a quota (sets archived_at). A repeat archive is a no-op.
// Returns the full archived entity (200+entity, per the contract — never 204).
func (s *QuotaStore) Archive(ctx context.Context, id, workspaceID string) (Quota, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE quota SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil // already archived — idempotent, no audit
		}
		e := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeQuota, &id, nil, nil)
		e.WorkspaceID = workspaceID
		_, err = crmaudit.WriteTx(ctx, tx, e)
		return err
	})
	if err != nil {
		return Quota{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}
