package crmcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
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
func (s *DealStore) Create(ctx context.Context, d Deal, idempotencyKey string) (Deal, error) {
	if err := requireProvenance(d.Source, d.CapturedBy); err != nil {
		return Deal{}, err
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
		return Deal{}, err
	}
	return s.Get(ctx, d.ID, d.WorkspaceID)
}

// Get returns one deal by id, workspace-scoped; ErrNotFound if absent.
func (s *DealStore) Get(ctx context.Context, id, workspaceID string) (Deal, error) {
	var d Deal
	var stageEnteredAt sql.NullTime
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, name, pipeline_id, stage_id,
			       organization_id, owner_id, partner_org_id,
			       amount_minor, currency, fx_rate_to_base, fx_rate_date,
			       status, lost_reason, expected_close_date, closed_at,
			       forecast_category, wait_until, last_activity_at,
			       (`+stalledPredicate(3)+`) AS stalled,
			       version, source, captured_by, created_at, updated_at, archived_at,
			       (SELECT max(occurred_at) FROM deal_stage_history WHERE deal_id=deal.id) AS stage_entered_at,
			       (SELECT count(*) FROM relationship WHERE deal_id=deal.id AND kind='deal_stakeholder' AND archived_at IS NULL) AS stakeholder_count
			FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID, defaultStalledDays).Scan(
			&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID,
			&d.OrganizationID, &d.OwnerID, &d.PartnerOrgID,
			&d.AmountMinor, &d.Currency, &d.FxRateToBase, &d.FxRateDate,
			&d.Status, &d.LostReason, &d.ExpectedCloseDate, &d.ClosedAt,
			&d.ForecastCategory, &d.WaitUntil, &d.LastActivityAt, &d.Stalled,
			&d.Version, &d.Source, &d.CapturedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.ArchivedAt,
			&stageEnteredAt, &d.StakeholderCount,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return d, errs.ErrNotFound
	}
	if stageEnteredAt.Valid {
		d.StageEnteredAt = &stageEnteredAt.Time
	}
	return d, err
}

// FindByIdempotencyKey resolves a prior create-action audit row carrying the key
// in audit_log.evidence and returns the deal it created.
func (s *DealStore) FindByIdempotencyKey(ctx context.Context, workspaceID, key string) (Deal, bool, error) {
	if key == "" {
		return Deal{}, false, nil
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
		return Deal{}, false, nil
	}
	if err != nil {
		return Deal{}, false, err
	}
	d, err := s.Get(ctx, dealID, workspaceID)
	if err != nil {
		return Deal{}, false, err
	}
	return d, true, nil
}

// DealListFilter holds optional predicates for ListFiltered. Zero value = no extra filters.
type DealListFilter struct {
	PipelineID       string
	StageID          string
	OwnerID          string
	OrganizationID   string
	Status           string // "" | open | won | lost (validated by the caller)
	Stalled          bool
	ForecastCategory string
	PartnerOrgID     string
	PersonID         string
	Sort             string
}

// defaultStalledDays is the idle threshold for the stalled=true filter, matching
// the StalledDeals predicate in contextgraph.go which takes this value as a param.
const defaultStalledDays = 14

// stalledPredicateFmt is the single source of the deterministic "is this deal stalled"
// SQL rule: an open deal whose last_activity_at is NULL or older than the threshold.
// The %d placeholder is the bound param index carrying defaultStalledDays. Every site
// that decides staleness — the ?stalled=true filter predicate and the per-deal `stalled`
// projection on the Get/List reads — formats this one string with its own param index,
// so the filter and the per-deal flag agree by construction.
// The param is an integer day-count multiplied by a 1-day interval — NOT string
// concatenation. `($n || ' days')::interval` would type $n as text, which the pgx
// driver refuses to encode an int into (lib/pq coerces silently); integer × interval
// keeps $n integer-typed so the predicate is portable across both drivers.
const stalledPredicateFmt = `status='open' AND (last_activity_at IS NULL OR last_activity_at < now() - ($%d * interval '1 day'))`

// stalledPredicate renders stalledPredicateFmt for the given bound-param index.
func stalledPredicate(paramN int) string {
	return fmt.Sprintf(stalledPredicateFmt, paramN)
}

var dealSortColumns = map[string]bool{
	"created_at":          true,
	"updated_at":          true,
	"amount_minor":        true,
	"expected_close_date": true,
	"last_activity_at":    true,
}

func dealOrderBy(sort string) string {
	if sort == "" {
		return "ORDER BY id"
	}
	var clauses []string
	for _, f := range strings.Split(sort, ",") {
		f = strings.TrimSpace(f)
		dir := "ASC"
		col := f
		if strings.HasPrefix(f, "-") {
			dir = "DESC"
			col = f[1:]
		}
		if !dealSortColumns[col] {
			continue
		}
		clauses = append(clauses, col+" "+dir)
	}
	clauses = append(clauses, "id")
	return "ORDER BY " + strings.Join(clauses, ", ")
}

// List delegates to ListFiltered with a zero filter, preserving the existing signature.
func (s *DealStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]Deal, string, error) {
	return s.ListFiltered(ctx, workspaceID, cursor, limit, DealListFilter{})
}

// ListFiltered returns cursor-keyed, workspace-scoped deals matching f.
// Predicates are AND-ed; all filter values are bound params.
func (s *DealStore) ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, f DealListFilter) ([]Deal, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Start with the fixed base args: workspaceID, cursor, fetch-limit.
	args := []any{workspaceID, cursor, limit + 1}
	n := 3 // next $N index

	where := `workspace_id=$1::uuid AND archived_at IS NULL AND ($2 = '' OR id::text > $2)`

	if f.PipelineID != "" {
		n++
		args = append(args, f.PipelineID)
		where += fmt.Sprintf(` AND pipeline_id=$%d::uuid`, n)
	}
	if f.StageID != "" {
		n++
		args = append(args, f.StageID)
		where += fmt.Sprintf(` AND stage_id=$%d::uuid`, n)
	}
	if f.OwnerID != "" {
		n++
		args = append(args, f.OwnerID)
		where += fmt.Sprintf(` AND owner_id=$%d::uuid`, n)
	}
	if f.OrganizationID != "" {
		n++
		args = append(args, f.OrganizationID)
		where += fmt.Sprintf(` AND organization_id=$%d::uuid`, n)
	}
	if f.Status != "" {
		n++
		args = append(args, f.Status)
		where += fmt.Sprintf(` AND status=$%d`, n)
	}
	if f.Stalled {
		n++
		args = append(args, defaultStalledDays)
		where += ` AND ` + stalledPredicate(n)
	}
	if f.ForecastCategory != "" {
		n++
		args = append(args, f.ForecastCategory)
		where += fmt.Sprintf(` AND forecast_category=$%d`, n)
	}
	if f.PartnerOrgID != "" {
		n++
		args = append(args, f.PartnerOrgID)
		where += fmt.Sprintf(` AND partner_org_id=$%d::uuid`, n)
	}
	if f.PersonID != "" {
		n++
		args = append(args, f.PersonID)
		where += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM relationship WHERE relationship.deal_id=deal.id AND relationship.kind='deal_stakeholder' AND relationship.person_id=$%d::uuid AND relationship.archived_at IS NULL)`, n)
	}
	stalledN := len(args) + 1
	args = append(args, defaultStalledDays)

	out := []Deal{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		//nolint:gosec // G202: `where` and stalledPredicate inject only bound-param indices ($N), never user input; all filter values are passed via args
		rows, err := tx.QueryContext(ctx,
			`SELECT id, workspace_id, name, pipeline_id, stage_id,
			        organization_id, owner_id,
			        amount_minor, currency, status, last_activity_at,
			        (`+stalledPredicate(stalledN)+`) AS stalled,
			        version, source, captured_by, created_at, updated_at,
			        (SELECT max(occurred_at) FROM deal_stage_history WHERE deal_id=deal.id) AS stage_entered_at,
			        (SELECT count(*) FROM relationship WHERE deal_id=deal.id AND kind='deal_stakeholder' AND archived_at IS NULL) AS stakeholder_count
			 FROM deal
			 WHERE `+where+`
			 `+dealOrderBy(f.Sort)+` LIMIT $3`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var d Deal
			var stageEnteredAt sql.NullTime
			if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID,
				&d.OrganizationID, &d.OwnerID,
				&d.AmountMinor, &d.Currency, &d.Status, &d.LastActivityAt, &d.Stalled, &d.Version,
				&d.Source, &d.CapturedBy,
				&d.CreatedAt, &d.UpdatedAt,
				&stageEnteredAt, &d.StakeholderCount); err != nil {
				return err
			}
			if stageEnteredAt.Valid {
				d.StageEnteredAt = &stageEnteredAt.Time
			}
			out = append(out, d)
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

// Update applies partial updates. When status moves to won/lost it freezes the FX rate.
func (s *DealStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (Deal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Deal{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Deal{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return Deal{}, err
	}

	if stageID, ok := updates["stage_id"].(string); ok && stageID != "" {
		var pipelineID string
		if err := tx.QueryRowContext(ctx, `SELECT pipeline_id FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&pipelineID); err != nil {
			return Deal{}, err
		}
		var inPipeline bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM stage
				WHERE id=$1::uuid AND pipeline_id=$2::uuid AND workspace_id=$3::uuid AND archived_at IS NULL
			)`,
			stageID, pipelineID, workspaceID).Scan(&inPipeline); err != nil {
			return Deal{}, err
		}
		if !inPipeline {
			return Deal{}, errs.ErrStageNotInPipeline
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
		    updated_at          = now()
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		  AND ($11 = 0 OR version = $11)`,
		id, workspaceID,
		nullStr(updates, "name"),
		nullStr(updates, "stage_id"),
		nullStr(updates, "status"),
		nullStr(updates, "lost_reason"),
		fxRate,
		fxRateDate,
		nullStr(updates, "expected_close_date"),
		nullStr(updates, "owner_id"),
		ifMatch)
	if err != nil {
		return Deal{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if ifMatch != 0 {
			return Deal{}, errs.ErrVersionSkew
		}
		return Deal{}, errs.ErrNotFound
	}

	// If stage changed, write deal_stage_history
	if stageID := nullStr(updates, "stage_id"); stageID != nil {
		var fromStageID string
		_ = tx.QueryRowContext(ctx, `SELECT stage_id FROM deal WHERE id=$1::uuid`, id).Scan(&fromStageID)
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id, changed_by)
			VALUES ($1::uuid, $2::uuid, NULLIF($3,'')::uuid, $4::uuid, $5)`,
			workspaceID, id, fromStageID, *stageID, workspaceID)
	}

	if err := tx.Commit(); err != nil {
		return Deal{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// freezeDealFX returns the latest FX rate (and its date) for the deal's current
// currency when the status is moving to won/lost — the rate to freeze onto the deal.
// Both are nil when the deal is not closing, has no currency, or has no FX rate on file;
// the caller COALESCEs them so a nil leaves the stored value untouched.
func (s *DealStore) freezeDealFX(ctx context.Context, tx *sql.Tx, workspaceID, id, newStatus string) (*float64, *time.Time) {
	if newStatus != statusWon && newStatus != statusLost {
		return nil, nil
	}
	var currency sql.NullString
	_ = tx.QueryRowContext(ctx, `SELECT currency FROM deal WHERE id=$1::uuid`, id).Scan(&currency)
	if !currency.Valid || currency.String == "" {
		return nil, nil
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

// AdvanceStage moves a deal to a new stage in a transaction that also writes deal_stage_history.
func (s *DealStore) AdvanceStage(ctx context.Context, id, workspaceID, toStageID, changedBy string) (Deal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Deal{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return Deal{}, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return Deal{}, err
	}

	var fromStageID string
	var amountMinor sql.NullInt64
	var currency sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT stage_id, amount_minor, currency FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
		id, workspaceID).Scan(&fromStageID, &amountMinor, &currency)
	if errors.Is(err, sql.ErrNoRows) {
		return Deal{}, errs.ErrNotFound
	}
	if err != nil {
		return Deal{}, err
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE deal SET stage_id=$1::uuid, updated_at=now() WHERE id=$2::uuid AND workspace_id=$3::uuid AND archived_at IS NULL`,
		toStageID, id, workspaceID)
	if err != nil {
		return Deal{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Deal{}, errs.ErrNotFound
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO deal_stage_history (workspace_id, deal_id, from_stage_id, to_stage_id,
		    changed_by, amount_minor_at_change, currency_at_change)
		VALUES ($1::uuid, $2::uuid, NULLIF($3,'')::uuid, $4::uuid, $5, $6, $7)`,
		workspaceID, id, fromStageID, toStageID, changedBy,
		amountMinor, currency)
	if err != nil {
		return Deal{}, err
	}

	// Resolve the target stage semantic to decide on closed-won status + outbox event.
	var toSemantic string
	if err = tx.QueryRowContext(ctx,
		`SELECT semantic FROM stage WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		toStageID, workspaceID).Scan(&toSemantic); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Deal{}, err
	}

	if toSemantic == statusWon {
		if _, err = tx.ExecContext(ctx,
			`UPDATE deal SET status='won', closed_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID); err != nil {
			return Deal{}, err
		}
		payload, _ := json.Marshal(map[string]any{
			"to_status":   statusWon,
			colDealID:     id,
			"to_stage_id": toStageID,
		})
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "deal.stage_changed", id, payload); err != nil {
			return Deal{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Deal{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes a deal (sets archived_at).
func (s *DealStore) Archive(ctx context.Context, id, workspaceID string) (Deal, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE deal SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		return err
	})
	if err != nil {
		return Deal{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches a deal by id regardless of archived_at status.
func (s *DealStore) getAny(ctx context.Context, id, workspaceID string) (Deal, error) {
	var d Deal
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, name, pipeline_id, stage_id,
			       organization_id, owner_id, partner_org_id,
			       amount_minor, currency, fx_rate_to_base, fx_rate_date,
			       status, lost_reason, expected_close_date, closed_at,
			       forecast_category, wait_until, last_activity_at,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID,
			&d.OrganizationID, &d.OwnerID, &d.PartnerOrgID,
			&d.AmountMinor, &d.Currency, &d.FxRateToBase, &d.FxRateDate,
			&d.Status, &d.LostReason, &d.ExpectedCloseDate, &d.ClosedAt,
			&d.ForecastCategory, &d.WaitUntil, &d.LastActivityAt,
			&d.Version, &d.Source, &d.CapturedBy,
			&d.CreatedAt, &d.UpdatedAt, &d.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return d, errs.ErrNotFound
	}
	return d, err
}
