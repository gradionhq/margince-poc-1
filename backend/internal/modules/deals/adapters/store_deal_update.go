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
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

// ---------------------------------------------------------------------------
// DealStore — Update (partial update, stage-in-pipeline guard, FX freeze on
// close, partner_org_id reassignment audit) and its private helpers.
// ---------------------------------------------------------------------------

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
	if stageID, ok := updates[fieldStageID].(string); ok && stageID != "" {
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

	newStatus, _ := updates[fieldStatus].(string)

	// If closing (won/lost), freeze the FX rate against the deal's current currency.
	fxRate, fxRateDate := s.freezeDealFX(ctx, tx, workspaceID, id, newStatus)

	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, entityTypeDeal)
	if err != nil {
		return err
	}

	args := []any{
		id, workspaceID,
		sqlutil.NullStr(updates, "name"),
		sqlutil.NullStr(updates, fieldStageID),
		sqlutil.NullStr(updates, fieldStatus),
		sqlutil.NullStr(updates, "lost_reason"),
		fxRate,
		fxRateDate,
		sqlutil.NullStr(updates, "expected_close_date"),
		sqlutil.NullStr(updates, "owner_id"),
		sqlutil.NullStr(updates, fieldPartnerOrgID),
	}
	customSet, args, n := dealUpdateCustomSet(updates, active, args, len(args))
	n++
	args = append(args, ifMatch)
	ifMatchIdx := n

	// The optimistic-concurrency guard is folded into the WHERE: ifMatch==0 skips the
	// version check (last-write-wins); a non-zero ifMatch requires the row version to match.
	res, err := tx.ExecContext(ctx, fmt.Sprintf(`
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
		    partner_org_id      = COALESCE($11::uuid, partner_org_id)%s,
		    updated_at          = now()
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		  AND ($%d = 0 OR version = $%d)`, customSet, ifMatchIdx, ifMatchIdx),
		args...)
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
	if newPartnerOrgID := sqlutil.NullStr(updates, fieldPartnerOrgID); partnerOrgIDProvided && newPartnerOrgID != nil {
		if err := s.auditPartnerOrgIDChange(ctx, tx, id, workspaceID, priorPartnerOrgID, *newPartnerOrgID); err != nil {
			return err
		}
	}
	return nil
}

// dealUpdateCustomSet appends one `<quoted col> = $N` clause per active custom
// column present as a key in updates (value converted via customfields.SQLValue;
// a shape mismatch simply skips that key, mirroring Create's rawExtra handling),
// returning the SET-clause fragment (empty, or comma-prefixed), the extended
// args slice, and the next $N index.
func dealUpdateCustomSet(updates map[string]any, active []customfields.Column, args []any, n int) (string, []any, int) {
	var clauses []string
	for _, c := range active {
		v, ok := updates[c.ColumnName]
		if !ok {
			continue
		}
		val, ok := customfields.SQLValue(c, v)
		if !ok {
			continue
		}
		n++
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(c.ColumnName), n))
	}
	if len(clauses) == 0 {
		return "", args, n
	}
	return ", " + strings.Join(clauses, ", "), args, n
}

// writeStageHistoryOnChange inserts a deal_stage_history row when updates
// carries a non-empty stage_id, recording the deal's stage before the UPDATE
// ran as from_stage_id. Best-effort: history-write failures do not fail the
// surrounding update, matching the pre-refactor inline behavior.
func (s *DealStore) writeStageHistoryOnChange(ctx context.Context, tx *sql.Tx, id, workspaceID string, updates map[string]any) {
	stageID := sqlutil.NullStr(updates, fieldStageID)
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
