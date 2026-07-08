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
)

// Advance moves a deal to ToStageID, deriving status from the target stage's
// semantic (DEAL-WIRE-9). An explicit mismatching Status is rejected with
// ErrStatusMismatch. Writes exactly one deal_stage_history row, one audit_log
// row (action=advance_stage), and one deal.stage_changed event per call.
// FX freeze on close (DM-FX-3) and reopen clear are handled here.
//
//nolint:cyclop // transactional advance path: lock, validate, write history/audit/event in one tx — branching is inherent
func (s *DealStore) Advance(ctx context.Context, id, workspaceID string, in domain.AdvanceInput, ifMatch int64, changedBy string) (domain.Deal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Deal{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return domain.Deal{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return domain.Deal{}, err
	}

	var fromStageID, fromStatus string
	var amountMinor sql.NullInt64
	var currency sql.NullString
	var version int64
	err = tx.QueryRowContext(ctx, `
		SELECT stage_id, status, amount_minor, currency, version
		FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		FOR UPDATE`,
		id, workspaceID).Scan(&fromStageID, &fromStatus, &amountMinor, &currency, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Deal{}, errs.ErrNotFound
	}
	if err != nil {
		return domain.Deal{}, err
	}
	if ifMatch != 0 && version != ifMatch {
		return domain.Deal{}, errs.ErrVersionSkew
	}

	if err := s.checkStageInPipeline(ctx, tx, id, workspaceID, in.ToStageID); err != nil {
		return domain.Deal{}, err
	}

	var toSemantic string
	if err := tx.QueryRowContext(ctx,
		`SELECT semantic FROM stage WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		in.ToStageID, workspaceID).Scan(&toSemantic); err != nil {
		return domain.Deal{}, err
	}

	if in.Status != "" && in.Status != toSemantic {
		return domain.Deal{}, errs.ErrStatusMismatch
	}
	if toSemantic == statusLost && (in.LostReason == nil || *in.LostReason == "") {
		return domain.Deal{}, errs.ErrLostReasonRequired
	}

	closing := toSemantic == statusWon || toSemantic == statusLost
	reopening := fromStatus != statusOpen && toSemantic == statusOpen

	var lostReason *string
	var fxRate *float64
	var fxRateDate *time.Time
	if closing {
		if toSemantic == statusLost {
			lostReason = in.LostReason
		}
		fxRate, fxRateDate = s.freezeDealFX(ctx, tx, workspaceID, id, toSemantic)
	}

	if err := advanceUpdate(ctx, tx, id, workspaceID, in.ToStageID, toSemantic,
		reopening, closing, lostReason, fxRate, fxRateDate, ifMatch); err != nil {
		return domain.Deal{}, err
	}

	if err := s.advanceSideEffects(ctx, tx, workspaceID, id, fromStageID, fromStatus, in.ToStageID, toSemantic, changedBy, amountMinor, currency); err != nil {
		return domain.Deal{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Deal{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// advanceSideEffects writes the deal_stage_history row, audit_log entry, and
// deal.stage_changed event for one advance gesture — all inside the caller's tx.
func (s *DealStore) advanceSideEffects(ctx context.Context, tx *sql.Tx, workspaceID, id, fromStageID, fromStatus, toStageID, toSemantic, changedBy string, amountMinor sql.NullInt64, currency sql.NullString) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id,
		    changed_by, amount_minor_at_change, currency_at_change)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7)`,
		workspaceID, id, fromStageID, toStageID, changedBy, amountMinor, currency); err != nil {
		return fmt.Errorf("deal advance history: %w", err)
	}

	e := crmaudit.EntryFromPrincipal(ctx, "advance_stage", entityTypeDeal, &id,
		map[string]any{"stage_id": fromStageID, "status": fromStatus},
		map[string]any{"stage_id": toStageID, "status": toSemantic})
	e.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
		return fmt.Errorf("deal advance audit: %w", err)
	}

	var winProbability int
	_ = tx.QueryRowContext(ctx, `SELECT win_probability FROM stage WHERE id=$1::uuid`, toStageID).Scan(&winProbability)

	var amountMinorPtr *int64
	if amountMinor.Valid {
		amountMinorPtr = &amountMinor.Int64
	}
	var currencyPtr *string
	if currency.Valid {
		currencyPtr = &currency.String
	}

	payload, _ := json.Marshal(map[string]any{
		colDealID:         id,
		"from_stage_id":   fromStageID,
		"to_stage_id":     toStageID,
		"from_status":     fromStatus,
		"to_status":       toSemantic,
		"amount_minor":    amountMinorPtr,
		"currency":        currencyPtr,
		"win_probability": winProbability,
	})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "deal.stage_changed", id, payload); err != nil {
		return fmt.Errorf("deal advance event: %w", err)
	}
	return nil
}

// advanceUpdate executes the deal UPDATE for one advance gesture and returns
// ErrVersionSkew or ErrNotFound if the row was not mutated.
func advanceUpdate(ctx context.Context, tx *sql.Tx, id, workspaceID, toStageID, toSemantic string, reopening, closing bool, lostReason *string, fxRate *float64, fxRateDate *time.Time, ifMatch int64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE deal
		SET stage_id        = $3::uuid,
		    status          = $4,
		    lost_reason     = CASE WHEN $5 THEN NULL WHEN $6 THEN $7 ELSE lost_reason END,
		    closed_at       = CASE WHEN $5 THEN NULL WHEN $6 THEN now() ELSE closed_at END,
		    fx_rate_to_base = CASE WHEN $5 THEN NULL WHEN $6 THEN COALESCE($8, fx_rate_to_base) ELSE fx_rate_to_base END,
		    fx_rate_date    = CASE WHEN $5 THEN NULL WHEN $6 THEN COALESCE($9, fx_rate_date) ELSE fx_rate_date END,
		    updated_at      = now()
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		  AND ($10 = 0 OR version = $10)`,
		id, workspaceID, toStageID, toSemantic,
		reopening, closing, lostReason, fxRate, fxRateDate, ifMatch)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if ifMatch != 0 {
			return errs.ErrVersionSkew
		}
		return errs.ErrNotFound
	}
	return nil
}
