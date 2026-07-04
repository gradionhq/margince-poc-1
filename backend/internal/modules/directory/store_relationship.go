package crmcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// relKindEmployment and relKindDealStakeholder are the only two `kind` values
// this store's write surface accepts. The partner-org kinds
// (partner_of/referred_by/co_sell_with) are readable via List — T15/A41 owns
// their write surface (deal.partner_org_id), not this ticket (T08).
const (
	relKindEmployment      = "employment"
	relKindDealStakeholder = "deal_stakeholder"
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
func owningStream(rel Relationship) (entityType string, entityID *string) {
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

func scanRelationshipRow(row relationshipRowScanner) (Relationship, error) {
	var rel Relationship
	var startedAt, endedAt, archivedAt sql.NullTime
	if err := row.Scan(
		&rel.ID, &rel.WorkspaceID, &rel.Kind, &rel.PersonID, &rel.OrganizationID, &rel.DealID,
		&rel.CounterpartyOrgID, &rel.Role, &rel.IsCurrentPrimary, &startedAt, &endedAt,
		&rel.Version, &rel.Source, &rel.CapturedBy, &rel.CreatedAt, &rel.UpdatedAt, &archivedAt,
	); err != nil {
		return Relationship{}, err
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
func (s *RelationshipStore) Create(ctx context.Context, rel Relationship) (Relationship, error) {
	if err := requireProvenance(rel.Source, rel.CapturedBy); err != nil {
		return Relationship{}, err
	}
	rel.ID = ids.New()
	err := withWorkspaceTx(ctx, s.db, rel.WorkspaceID, func(tx *sql.Tx) error {
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
		payload, _ := json.Marshal(map[string]any{"relationship_id": rel.ID, fieldKind: rel.Kind})
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
		return Relationship{}, err
	}
	return s.Get(ctx, rel.ID, rel.WorkspaceID)
}

// Get returns one live relationship by id, workspace-scoped; ErrNotFound if
// absent or archived.
func (s *RelationshipStore) Get(ctx context.Context, id, workspaceID string) (Relationship, error) {
	var rel Relationship
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
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
