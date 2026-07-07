package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
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

// requireProvenance rejects an empty source or captured_by with a typed sentinel
// (data-model §1.6 provenance). HTTP handlers already reject empties at the edge, but
// non-HTTP callers (import/Datasource/direct store use) must not be able to insert source=""
// or captured_by="" — provenance is a load-bearing invariant, not a nicety.
func requireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

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
func (s *DealStore) Create(ctx context.Context, d domain.Deal, idempotencyKey string) (domain.Deal, error) {
	if err := requireProvenance(d.Source, d.CapturedBy); err != nil {
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

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id,
			    organization_id, owner_id, partner_org_id,
			    amount_minor, currency, status,
			    expected_close_date, forecast_category,
			    source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,1)`,
			d.ID, d.WorkspaceID, d.Name, d.PipelineID, d.StageID,
			d.OrganizationID, d.OwnerID, d.PartnerOrgID,
			d.AmountMinor, d.Currency, d.Status,
			d.ExpectedCloseDate, d.ForecastCategory,
			d.Source, d.CapturedBy); err != nil {
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
	return s.Get(ctx, d.ID, d.WorkspaceID)
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

// Update applies partial updates. When status moves to won/lost it freezes the FX rate.
func (s *DealStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Deal, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return s.applyUpdate(ctx, tx, id, workspaceID, updates, ifMatch)
	})
	if err != nil {
		return domain.Deal{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// applyUpdate runs the write side of Update inside an already-scoped tx (role
// + app.workspace_id GUC set by withWorkspaceTx): the stage-in-pipeline guard,
// the partner_org_id before/after capture, the FX freeze, the UPDATE itself,
// and the stage-history / partner-reassignment side effects. Split out of
// Update to keep the outer function's cognitive complexity within budget.
func (s *DealStore) applyUpdate(ctx context.Context, tx *sql.Tx, id, workspaceID string, updates map[string]any, ifMatch int64) error {
	if stageID, ok := updates["stage_id"].(string); ok && stageID != "" {
		if err := s.checkStageInPipeline(ctx, tx, id, workspaceID, stageID); err != nil {
			return err
		}
	}

	// Capture the previous partner_org_id before the row changes so a later
	// partner reassignment can be audited with both sides of the diff.
	_, partnerOrgIDProvided := updates[fieldPartnerOrgID]
	var priorPartnerOrgID sql.NullString
	if partnerOrgIDProvided {
		var err error
		priorPartnerOrgID, err = s.capturePriorPartnerOrgID(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
	}

	newStatus, _ := updates["status"].(string)

	// If closing (won/lost), freeze the FX rate against the deal's current currency.
	fxRate, fxRateDate := s.freezeDealFX(ctx, tx, workspaceID, id, newStatus)

	// The optimistic-concurrency guard is folded into the WHERE: ifMatch==0 skips the
	// version check (last-write-wins); a non-zero ifMatch requires the row version to match.
	res, err := tx.ExecContext(ctx, `
		UPDATE deal
		SET name                = COALESCE($3, name),
		    stage_id            = COALESCE($4::uuid, stage_id),
		    status              = COALESCE($5, status),
		    lost_reason         = COALESCE($6, lost_reason),
		    closed_at           = CASE WHEN $5 IN ('won','lost') THEN now() ELSE closed_at END,
		    fx_rate_to_base     = COALESCE($7, fx_rate_to_base),
		    fx_rate_date        = COALESCE($8, fx_rate_date),
		    expected_close_date = COALESCE($9, expected_close_date),
		    owner_id            = COALESCE($10::uuid, owner_id),
		    partner_org_id      = COALESCE($11::uuid, partner_org_id),
		    updated_at          = now()
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		  AND ($12 = 0 OR version = $12)`,
		id, workspaceID,
		nullStr(updates, "name"),
		nullStr(updates, "stage_id"),
		nullStr(updates, "status"),
		nullStr(updates, "lost_reason"),
		fxRate,
		fxRateDate,
		nullStr(updates, "expected_close_date"),
		nullStr(updates, "owner_id"),
		nullStr(updates, fieldPartnerOrgID),
		ifMatch)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if ifMatch != 0 {
			return errs.ErrVersionSkew
		}
		return errs.ErrNotFound
	}

	s.writeStageHistoryOnChange(ctx, tx, id, workspaceID, updates)

	// Only write the partner reassignment side effects when the supplied value
	// actually changes. That keeps the audit/outbox noise aligned with the field.
	if newPartnerOrgID := nullStr(updates, fieldPartnerOrgID); partnerOrgIDProvided && newPartnerOrgID != nil {
		if err := s.auditPartnerOrgIDChange(ctx, tx, id, workspaceID, priorPartnerOrgID, *newPartnerOrgID); err != nil {
			return err
		}
	}
	return nil
}

// writeStageHistoryOnChange inserts a deal_stage_history row when updates
// carries a non-empty stage_id, recording the deal's stage before the UPDATE
// ran as from_stage_id. Best-effort: history-write failures do not fail the
// surrounding update, matching the pre-refactor inline behavior.
func (s *DealStore) writeStageHistoryOnChange(ctx context.Context, tx *sql.Tx, id, workspaceID string, updates map[string]any) {
	stageID := nullStr(updates, "stage_id")
	if stageID == nil {
		return
	}
	var fromStageID string
	_ = tx.QueryRowContext(ctx, `SELECT stage_id FROM deal WHERE id=$1::uuid`, id).Scan(&fromStageID)
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id, changed_by)
		VALUES ($1::uuid, $2::uuid, NULLIF($3,'')::uuid, $4::uuid, $5)`,
		workspaceID, id, fromStageID, *stageID, workspaceID)
}

// capturePriorPartnerOrgID reads the deal's current partner_org_id inside tx,
// before the UPDATE runs, so a later partner reassignment can be audited with
// both sides of the diff. sql.ErrNoRows is treated as "no prior value" rather
// than an error since the row-existence check happens later, in the UPDATE.
func (s *DealStore) capturePriorPartnerOrgID(ctx context.Context, tx *sql.Tx, id, workspaceID string) (sql.NullString, error) {
	var prior sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT partner_org_id FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&prior); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sql.NullString{}, err
	}
	return prior, nil
}

// auditPartnerOrgIDChange writes the deal.partner_assigned outbox event and
// the matching audit_log row when newVal differs from prior. It is a no-op
// when the value is unchanged, keeping the audit/outbox noise aligned with
// the field actually changing.
func (s *DealStore) auditPartnerOrgIDChange(ctx context.Context, tx *sql.Tx, id, workspaceID string, prior sql.NullString, newVal string) error {
	priorStr := ""
	if prior.Valid {
		priorStr = prior.String
	}
	if priorStr == newVal {
		return nil
	}

	payload, _ := json.Marshal(map[string]any{"deal_id": id, fieldPartnerOrgID: newVal})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "deal.partner_assigned", id, payload); err != nil {
		return fmt.Errorf("deal update partner event: %w", err)
	}

	before := map[string]any{fieldPartnerOrgID: prior.String}
	if !prior.Valid {
		before = map[string]any{fieldPartnerOrgID: nil}
	}
	after := map[string]any{fieldPartnerOrgID: newVal}
	e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeDeal, &id, before, after)
	e.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
		return fmt.Errorf("deal update partner audit: %w", err)
	}
	return nil
}

// checkStageInPipeline verifies that stageID belongs to the pipeline the deal
// identified by id currently sits in, returning errs.ErrStageNotInPipeline if not.
func (s *DealStore) checkStageInPipeline(ctx context.Context, tx *sql.Tx, id, workspaceID, stageID string) error {
	var pipelineID string
	if err := tx.QueryRowContext(ctx, `SELECT pipeline_id FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		id, workspaceID).Scan(&pipelineID); err != nil {
		return err
	}
	var inPipeline bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM stage
			WHERE id=$1::uuid AND pipeline_id=$2::uuid AND workspace_id=$3::uuid AND archived_at IS NULL
		)`,
		stageID, pipelineID, workspaceID).Scan(&inPipeline); err != nil {
		return err
	}
	if !inPipeline {
		return errs.ErrStageNotInPipeline
	}
	return nil
}

// freezeDealFX returns the latest FX rate (and its date) for the deal's current
// currency when the status is moving to won/lost — the rate to freeze onto the deal.
// When the deal's currency matches the workspace's base_currency, the rate is a
// genuine 1:1 identity and is returned directly without consulting fx_rate: a
// workspace closing a deal denominated in its own base currency should never
// need an explicit identity conversion rate pre-seeded on file.
// Both are nil when the deal is not closing, has no currency, or (for a foreign
// currency) has no FX rate on file; the caller COALESCEs them so a nil leaves
// the stored value untouched.
func (s *DealStore) freezeDealFX(ctx context.Context, tx *sql.Tx, workspaceID, id, newStatus string) (*float64, *time.Time) {
	if newStatus != statusWon && newStatus != statusLost {
		return nil, nil
	}
	var currency sql.NullString
	_ = tx.QueryRowContext(ctx, `SELECT currency FROM deal WHERE id=$1::uuid`, id).Scan(&currency)
	if !currency.Valid || currency.String == "" {
		return nil, nil
	}

	var baseCurrency sql.NullString
	_ = tx.QueryRowContext(ctx, `SELECT base_currency FROM workspace WHERE id=$1::uuid`, workspaceID).Scan(&baseCurrency)
	if baseCurrency.Valid && baseCurrency.String == currency.String {
		rate := 1.0
		now := time.Now().UTC()
		return &rate, &now
	}

	var rate float64
	var rateDate time.Time
	if err := tx.QueryRowContext(ctx, `
		SELECT rate, rate_date FROM fx_rate
		WHERE workspace_id=$1::uuid AND from_currency=$2
		ORDER BY rate_date DESC LIMIT 1`,
		workspaceID, currency.String).Scan(&rate, &rateDate); err != nil {
		return nil, nil
	}
	return &rate, &rateDate
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
		query := `
			SELECT id, workspace_id, name, pipeline_id, stage_id,
			       organization_id, owner_id, partner_org_id,
			       amount_minor, currency, fx_rate_to_base, fx_rate_date,
			       status, lost_reason, expected_close_date, closed_at,
			       forecast_category, wait_until, last_activity_at,
			       version, source, captured_by, created_at, updated_at, archived_at,
			       (SELECT max(occurred_at) FROM deal_stage_history WHERE deal_id=deal.id) AS stage_entered_at,
			       (SELECT count(*) FROM relationship WHERE deal_id=deal.id AND kind='deal_stakeholder' AND archived_at IS NULL) AS stakeholder_count
			FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`
		if !includeArchived {
			query += " AND archived_at IS NULL"
		}
		return tx.QueryRowContext(ctx, query,
			id, workspaceID).Scan(
			&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID,
			&d.OrganizationID, &d.OwnerID, &d.PartnerOrgID,
			&d.AmountMinor, &d.Currency, &d.FxRateToBase, &d.FxRateDate,
			&d.Status, &d.LostReason, &d.ExpectedCloseDate, &d.ClosedAt,
			&d.ForecastCategory, &d.WaitUntil, &d.LastActivityAt,
			&d.Version, &d.Source, &d.CapturedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.ArchivedAt,
			&stageEnteredAt, &d.StakeholderCount,
		)
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
