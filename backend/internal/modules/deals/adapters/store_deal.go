package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

// ---------------------------------------------------------------------------
// Package-level constants shared across deal store files
// ---------------------------------------------------------------------------

const (
	entityTypeDeal    = "deal"
	statusOpen        = "open"
	statusWon         = "won"
	statusLost        = "lost"
	colDealID         = "deal_id"
	fieldPartnerOrgID = "partner_org_id"
)

// ---------------------------------------------------------------------------
// DealStore
// ---------------------------------------------------------------------------

// DealStore manages deal rows, including stage transitions and FX freeze.
type DealStore struct{ db *sql.DB }

// NewDealStore returns a DealStore.
func NewDealStore(db *sql.DB) *DealStore { return &DealStore{db: db} }

// Create inserts a new deal row, its initial stage history row, its create
// audit_log entry, and its deal.created outbox event in one workspace-scoped tx.
// The stage pre-check keeps the error readable at the store boundary instead of
// relying on the composite FK to surface a lower-level constraint violation.
func (s *DealStore) Create(ctx context.Context, d domain.Deal, idempotencyKey string, rawExtra map[string]any) (domain.Deal, error) {
	if err := sqlutil.RequireProvenance(d.Source, d.CapturedBy); err != nil {
		return domain.Deal{}, err
	}
	d.ID = ids.New()
	err := withWorkspaceTx(ctx, s.db, d.WorkspaceID, func(tx *sql.Tx) error {
		var inPipeline bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM stage
				WHERE id=$1::uuid AND pipeline_id=$2::uuid AND workspace_id=$3::uuid AND archived_at IS NULL
			)`,
			d.StageID, d.PipelineID, d.WorkspaceID).Scan(&inPipeline); err != nil {
			return err
		}
		if !inPipeline {
			return errs.ErrStageNotInPipeline
		}

		active, err := customfields.ActiveColumns(ctx, s.db, d.WorkspaceID, entityTypeDeal)
		if err != nil {
			return err
		}
		cols := []string{"id", "workspace_id", "name", "pipeline_id", "stage_id", "organization_id", "owner_id", "partner_org_id", "amount_minor", "currency", "status", "expected_close_date", "forecast_category", "source", "captured_by", "version"}
		args := []any{d.ID, d.WorkspaceID, d.Name, d.PipelineID, d.StageID, d.OrganizationID, d.OwnerID, d.PartnerOrgID, d.AmountMinor, d.Currency, d.Status, d.ExpectedCloseDate, d.ForecastCategory, d.Source, d.CapturedBy, 1}
		if rawExtra != nil {
			for _, c := range active {
				v, ok := rawExtra[c.ColumnName]
				if !ok {
					continue
				}
				if val, ok := customfields.SQLValue(c, v); ok {
					cols = append(cols, c.ColumnName)
					args = append(args, val)
				}
			}
		}
		placeholders := make([]string, len(cols))
		for i := range cols {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			`INSERT INTO deal (%s) VALUES (%s)`,
			strings.Join(cols, ", "), strings.Join(placeholders, ",")), args...); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deal_stage_history (
				workspace_id, deal_id, from_stage_id, to_stage_id,
				changed_by, amount_minor_at_change, currency_at_change
			)
			VALUES ($1::uuid, $2::uuid, NULL, $3::uuid, $4, $5, $6)`,
			d.WorkspaceID, d.ID, d.StageID, d.CapturedBy, d.AmountMinor, d.Currency); err != nil {
			return fmt.Errorf("deal create history: %w", err)
		}

		payload, _ := json.Marshal(map[string]any{"deal_id": d.ID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			d.WorkspaceID, "deal.created", d.ID, payload); err != nil {
			return fmt.Errorf("deal create event: %w", err)
		}

		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeDeal, &d.ID, nil, d)
		e.WorkspaceID = d.WorkspaceID
		if idempotencyKey != "" {
			e.Evidence = map[string]any{"idempotency_key": idempotencyKey}
		}
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("deal create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Deal{}, err
	}
	out, err := s.Get(ctx, d.ID, d.WorkspaceID)
	if err != nil {
		return domain.Deal{}, err
	}
	return out, nil
}

// Get returns one deal by id, workspace-scoped; ErrNotFound if absent.
func (s *DealStore) Get(ctx context.Context, id, workspaceID string) (domain.Deal, error) {
	return s.loadDeal(ctx, id, workspaceID, false)
}

// FindByIdempotencyKey resolves a prior create-action audit row carrying the key
// in audit_log.evidence and returns the deal it created.
func (s *DealStore) FindByIdempotencyKey(ctx context.Context, workspaceID, key string) (domain.Deal, bool, error) {
	if key == "" {
		return domain.Deal{}, false, nil
	}

	var dealID string
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT entity_id
			FROM audit_log
			WHERE workspace_id=$1::uuid
			  AND entity_type=$2
			  AND action='create'
			  AND evidence->>'idempotency_key' = $3
			ORDER BY occurred_at DESC
			LIMIT 1`,
			workspaceID, entityTypeDeal, key).Scan(&dealID)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Deal{}, false, nil
	}
	if err != nil {
		return domain.Deal{}, false, err
	}
	d, err := s.Get(ctx, dealID, workspaceID)
	if err != nil {
		return domain.Deal{}, false, err
	}
	return d, true, nil
}

// StageSemantic returns the semantic value of a stage. Returns errs.ErrNotFound if not found.
func (s *DealStore) StageSemantic(ctx context.Context, stageID, workspaceID string) (string, error) {
	var semantic string
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT semantic FROM stage WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			stageID, workspaceID).Scan(&semantic)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", errs.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return semantic, nil
}

// Archive soft-deletes a deal (sets archived_at).
func (s *DealStore) Archive(ctx context.Context, id, workspaceID string) (domain.Deal, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE deal SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("deal archive rows affected: %w", err)
		}
		if n > 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE attachment SET archived_at=now()
				 WHERE entity_type='deal' AND entity_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
				id, workspaceID); err != nil {
				return fmt.Errorf("deal archive attachment cascade: %w", err)
			}
			payload, _ := json.Marshal(map[string]string{colDealID: id})
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
				 VALUES ($1,$2,$3::uuid,$4)`,
				workspaceID, "deal.archived", id, payload); err != nil {
				return fmt.Errorf("deal archive event: %w", err)
			}
			e := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypeDeal, &id, nil, nil)
			e.WorkspaceID = workspaceID
			if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
				return fmt.Errorf("deal archive audit: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Deal{}, err
	}
	return s.loadDeal(ctx, id, workspaceID, true)
}

// Restore clears archived_at, restoring a deal to default list visibility.
// Refuses errs.ErrNotArchived if already live. Deals have no merged_into_id
// column and no dedupe-key unique index to preflight here.
func (s *DealStore) Restore(ctx context.Context, id, workspaceID string) (domain.Deal, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var archivedAt sql.NullTime
		if err := tx.QueryRowContext(ctx,
			`SELECT archived_at FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&archivedAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.ErrNotFound
			}
			return err
		}
		if !archivedAt.Valid {
			return errs.ErrNotArchived
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE deal SET archived_at=NULL WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{colDealID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "deal.restored", id, payload); err != nil {
			return fmt.Errorf("deal restore event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "restore", entityTypeDeal, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("deal restore audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Deal{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// GetAny fetches a deal by id regardless of archived_at status.
func (s *DealStore) GetAny(ctx context.Context, id, workspaceID string) (domain.Deal, error) {
	return s.loadDeal(ctx, id, workspaceID, true)
}

func (s *DealStore) loadDeal(ctx context.Context, id, workspaceID string, includeArchived bool) (domain.Deal, error) {
	var d domain.Deal
	var stageEnteredAt sql.NullTime
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, entityTypeDeal)
		if err != nil {
			return err
		}
		query := `
			SELECT id, workspace_id, name, pipeline_id, stage_id,
			       organization_id, owner_id, partner_org_id,
			       amount_minor, currency, fx_rate_to_base, fx_rate_date,
			       status, lost_reason, expected_close_date, closed_at,
			       forecast_category, wait_until, last_activity_at,
			       version, source, captured_by, created_at, updated_at, archived_at,
			       (SELECT max(occurred_at) FROM deal_stage_history WHERE deal_id=deal.id) AS stage_entered_at,
			       (SELECT count(*) FROM relationship WHERE deal_id=deal.id AND kind='deal_stakeholder' AND archived_at IS NULL) AS stakeholder_count`
		for _, c := range active {
			query += ", " + pq.QuoteIdentifier(c.ColumnName)
		}
		query += `
			FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`
		if !includeArchived {
			query += " AND archived_at IS NULL"
		}
		dests := []any{
			&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID,
			&d.OrganizationID, &d.OwnerID, &d.PartnerOrgID,
			&d.AmountMinor, &d.Currency, &d.FxRateToBase, &d.FxRateDate,
			&d.Status, &d.LostReason, &d.ExpectedCloseDate, &d.ClosedAt,
			&d.ForecastCategory, &d.WaitUntil, &d.LastActivityAt,
			&d.Version, &d.Source, &d.CapturedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.ArchivedAt,
			&stageEnteredAt, &d.StakeholderCount,
		}
		dests = append(dests, customfields.ScanDests(active)...)
		if err := tx.QueryRowContext(ctx, query, id, workspaceID).Scan(dests...); err != nil {
			return err
		}
		d.CustomFields = customfields.ExtractValues(active, dests[len(dests)-len(active):])
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return d, errs.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if stageEnteredAt.Valid {
		d.StageEnteredAt = &stageEnteredAt.Time
	}
	d.Stalled, _ = domain.IsStalled(d, time.Now().UTC())
	return d, nil
}
