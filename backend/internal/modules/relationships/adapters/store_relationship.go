// Package adapters contains the relationships module's PostgreSQL storage adapter.
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

// relKindEmployment and relKindDealStakeholder are the only two `kind` values
// this store's write surface accepts. The partner-org kinds
// (partner_of/referred_by/co_sell_with) are readable via List — T15/A41 owns
// their write surface (deal.partner_org_id), not this ticket (T08).
const (
	relKindEmployment      = "employment"
	relKindDealStakeholder = "deal_stakeholder"

	// fieldRelationshipID is the event_outbox payload key for the
	// relationship id, shared by Create/Update/Archive.
	fieldRelationshipID = "relationship_id"
	fieldKind           = "kind"

	entityTypeDeal   = "deal"
	entityTypePerson = "person"
)

// RelationshipStore executes parameterized SQL against the relationship table
// (PO-DDL-7): employment (person<->org) and deal_stakeholder (deal<->person)
// writes, plus a generic read across every kind including the partner-org ones.
type RelationshipStore struct{ db *sql.DB }

// NewRelationshipStore returns a RelationshipStore backed by db.
func NewRelationshipStore(db *sql.DB) *RelationshipStore { return &RelationshipStore{db: db} }

// owningStream returns the audit_log/event_outbox entity_type + entity_id a
// relationship write is recorded against (GATE-CORE-3/5): employment writes
// land on the person stream, deal_stakeholder writes on the deal stream —
// never on a "relationship" stream of their own.
func owningStream(rel domain.Relationship) (entityType string, entityID *string) {
	if rel.Kind == relKindDealStakeholder {
		return entityTypeDeal, rel.DealID
	}
	return entityTypePerson, rel.PersonID
}

func relTopic(kind, verb string) string {
	if kind == relKindDealStakeholder {
		return "deal.stakeholder_" + verb
	}
	return "person.employment_" + verb
}

// mapRelationshipUniqueViolation maps the two relationship unique-index
// violations to errs.ErrConflict: uq_rel_current_primary_employer (PO-AC-12 —
// a second is_current_primary=true employment for the same person 409s rather
// than auto-demoting the prior row, which stays untouched as additive
// history) and uq_rel_deal_person_role (DEAL-AC-9 — a duplicate
// (deal_id,person_id,role) stakeholder row 409s).
func mapRelationshipUniqueViolation(err error) error {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.Constraint == "uq_rel_current_primary_employer" || pgErr.Constraint == "uq_rel_deal_person_role") {
		return errs.ErrConflict
	}
	return err
}

type relationshipRowScanner interface {
	Scan(dest ...any) error
}

func scanRelationshipRow(row relationshipRowScanner) (domain.Relationship, error) {
	var rel domain.Relationship
	var startedAt, endedAt, archivedAt sql.NullTime
	if err := row.Scan(
		&rel.ID, &rel.WorkspaceID, &rel.Kind, &rel.PersonID, &rel.OrganizationID, &rel.DealID,
		&rel.CounterpartyOrgID, &rel.Role, &rel.IsCurrentPrimary, &startedAt, &endedAt,
		&rel.Version, &rel.Source, &rel.CapturedBy, &rel.CreatedAt, &rel.UpdatedAt, &archivedAt,
	); err != nil {
		return domain.Relationship{}, err
	}
	if startedAt.Valid {
		t := startedAt.Time
		rel.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		rel.EndedAt = &t
	}
	if archivedAt.Valid {
		t := archivedAt.Time
		rel.ArchivedAt = &t
	}
	return rel, nil
}

const relationshipSelectCols = `
	id, workspace_id, kind, person_id, organization_id, deal_id,
	counterparty_org_id, role, is_primary, started_at, ended_at,
	version, source, captured_by, created_at, updated_at, archived_at`

// Create inserts a relationship row, its create audit_log entry, and its
// owning-stream outbox event in one workspace-scoped tx. Rejects a duplicate
// current-primary employer or a duplicate (deal_id,person_id,role) stakeholder
// as errs.ErrConflict (409) rather than silently demoting/overwriting — the
// chosen PO-AC-12 behavior: historical employment rows are additive, never
// auto-demoted.
func (s *RelationshipStore) Create(ctx context.Context, rel domain.Relationship) (domain.Relationship, error) {
	if err := sqlutil.RequireProvenance(rel.Source, rel.CapturedBy); err != nil {
		return domain.Relationship{}, err
	}
	rel.ID = ids.New()
	err := database.WithWorkspaceTx(ctx, s.db, rel.WorkspaceID, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO relationship (
			    id, workspace_id, kind, person_id, organization_id, deal_id,
			    counterparty_org_id, role, is_primary, started_at, ended_at,
			    source, captured_by, version
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,1)`,
			rel.ID, rel.WorkspaceID, rel.Kind, rel.PersonID, rel.OrganizationID, rel.DealID,
			rel.CounterpartyOrgID, rel.Role, rel.IsCurrentPrimary, rel.StartedAt, rel.EndedAt,
			rel.Source, rel.CapturedBy); err != nil {
			if mapped := mapRelationshipUniqueViolation(err); errors.Is(mapped, errs.ErrConflict) {
				return errs.ErrConflict
			}
			return fmt.Errorf("relationship create: %w", err)
		}

		entityType, entityID := owningStream(rel)
		payload, _ := json.Marshal(map[string]any{fieldRelationshipID: rel.ID, fieldKind: rel.Kind})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			rel.WorkspaceID, relTopic(rel.Kind, "created"), entityID, payload); err != nil {
			return fmt.Errorf("relationship create event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityType, entityID, nil, rel)
		e.WorkspaceID = rel.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("relationship create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Relationship{}, err
	}
	return s.Get(ctx, rel.ID, rel.WorkspaceID)
}

// Get returns one live relationship by id, workspace-scoped; ErrNotFound if
// absent or archived.
func (s *RelationshipStore) Get(ctx context.Context, id, workspaceID string) (domain.Relationship, error) {
	var rel domain.Relationship
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT `+relationshipSelectCols+` FROM relationship
			 WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		var scanErr error
		rel, scanErr = scanRelationshipRow(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return rel, errs.ErrNotFound
	}
	return rel, err
}

// List returns a cursor-paginated slice of relationship rows.
func (s *RelationshipStore) List(ctx context.Context, workspaceID, cursor string, limit int, filter domain.RelationshipListFilter) ([]domain.Relationship, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	out := []domain.Relationship{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID, cursor, limit + 1}
		where := ""
		if !filter.IncludeArchived {
			where += " AND archived_at IS NULL"
		}
		if filter.Kind != "" {
			args = append(args, filter.Kind)
			where += fmt.Sprintf(" AND kind=$%d", len(args))
		}
		if filter.PersonID != "" {
			args = append(args, filter.PersonID)
			where += fmt.Sprintf(" AND person_id=$%d::uuid", len(args))
		}
		if filter.OrganizationID != "" {
			args = append(args, filter.OrganizationID)
			where += fmt.Sprintf(" AND organization_id=$%d::uuid", len(args))
		}
		if filter.DealID != "" {
			args = append(args, filter.DealID)
			where += fmt.Sprintf(" AND deal_id=$%d::uuid", len(args))
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT `+relationshipSelectCols+`
			FROM relationship
			WHERE workspace_id=$1::uuid
			  AND ($2 = '' OR id::text > $2)`+where+`
			ORDER BY id LIMIT $3`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			rel, scanErr := scanRelationshipRow(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, rel)
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

// Update applies partial updates to a relationship using standard If-Match
// optimistic concurrency. Duplicate primary-employer and stakeholder-role
// conflicts map to ErrConflict exactly like Create.
func (s *RelationshipStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Relationship, error) {
	var kind string
	var personID, dealID *string
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			UPDATE relationship
			SET role       = COALESCE($3, role),
			    is_primary = COALESCE($4, is_primary),
			    started_at = COALESCE($5::date, started_at),
			    ended_at   = COALESCE($6::date, ended_at),
			    updated_at = now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
			  AND ($7 = 0 OR version = $7)
			RETURNING kind, person_id, deal_id`,
			id, workspaceID,
			sqlutil.NullStr(updates, "role"),
			nullBool(updates, "is_current_primary"),
			nullTime(updates, "started_at"),
			nullTime(updates, "ended_at"),
			ifMatch).Scan(&kind, &personID, &dealID)
		if errors.Is(err, sql.ErrNoRows) {
			if ifMatch != 0 {
				return errs.ErrVersionSkew
			}
			return errs.ErrNotFound
		}
		if err != nil {
			if mapped := mapRelationshipUniqueViolation(err); errors.Is(mapped, errs.ErrConflict) {
				return errs.ErrConflict
			}
			return fmt.Errorf("relationship update: %w", err)
		}

		rel := domain.Relationship{Kind: kind, PersonID: personID, DealID: dealID}
		entityType, entityID := owningStream(rel)
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityType, entityID, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("relationship update audit: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{fieldRelationshipID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, relTopic(kind, "updated"), entityID, payload); err != nil {
			return fmt.Errorf("relationship update event: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Relationship{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes a relationship and returns the archived row. Repeating
// the archive is a no-op.
func (s *RelationshipStore) Archive(ctx context.Context, id, workspaceID string) (domain.Relationship, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var kind string
		var personID, dealID *string
		if err := tx.QueryRowContext(ctx,
			`SELECT kind, person_id, deal_id FROM relationship WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&kind, &personID, &dealID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.ErrNotFound
			}
			return err
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE relationship SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}

		rel := domain.Relationship{Kind: kind, PersonID: personID, DealID: dealID}
		entityType, entityID := owningStream(rel)
		e := crmaudit.EntryFromPrincipal(ctx, "archive", entityType, entityID, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("relationship archive audit: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{fieldRelationshipID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, relTopic(kind, "archived"), entityID, payload); err != nil {
			return fmt.Errorf("relationship archive event: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Relationship{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches a relationship by id regardless of archived_at status.
func (s *RelationshipStore) getAny(ctx context.Context, id, workspaceID string) (domain.Relationship, error) {
	var rel domain.Relationship
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT `+relationshipSelectCols+` FROM relationship
			 WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID)
		var scanErr error
		rel, scanErr = scanRelationshipRow(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return rel, errs.ErrNotFound
	}
	return rel, err
}

// nullBool reads a bool from updates map.
func nullBool(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return nil
}

// nullTime reads a time.Time from updates map.
func nullTime(m map[string]any, key string) *time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case *time.Time:
			return t
		case time.Time:
			return &t
		case string:
			parsed, err := time.Parse(time.RFC3339, t)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}
