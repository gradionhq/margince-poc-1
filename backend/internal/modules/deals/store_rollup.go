package deals

import (
	"context"
	"database/sql"
	"errors"
	"time"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// RollupStore computes GET /pipelines/{id}/rollup over live open deals.
type RollupStore struct{ db *sql.DB }

// NewRollupStore returns a RollupStore.
func NewRollupStore(db *sql.DB) *RollupStore { return &RollupStore{db: db} }

// PipelineRollup is the wire shape of crm.yaml's PipelineRollup schema.
type PipelineRollup struct {
	PipelineID      string                `json:"pipeline_id"`
	UnweightedMinor int64                 `json:"unweighted_minor"`
	WeightedMinor   int64                 `json:"weighted_minor"`
	BaseCurrency    string                `json:"base_currency"`
	AsOfDate        string                `json:"as_of_date"`
	ByStage         []PipelineRollupStage `json:"by_stage"`
	Breakdown       []RollupDealRow       `json:"breakdown"`
}

// PipelineRollupStage is the wire shape of crm.yaml's PipelineRollupStage schema.
type PipelineRollupStage struct {
	StageID         string `json:"stage_id"`
	UnweightedMinor int64  `json:"unweighted_minor"`
	WeightedMinor   int64  `json:"weighted_minor"`
	DealCount       int    `json:"deal_count"`
}

type rollupDealRow struct {
	dealID         string
	stageID        string
	amountMinor    sql.NullInt64
	currency       sql.NullString
	winProbability int
}

// Get computes the roll-up for one pipeline as of asOf.
func (s *RollupStore) Get(ctx context.Context, pipelineID, workspaceID string, asOf time.Time) (PipelineRollup, error) {
	var out PipelineRollup
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		baseCurrency, err := s.loadRollupBaseCurrency(ctx, tx, workspaceID)
		if err != nil {
			return err
		}
		if err := s.ensureRollupPipelineExists(ctx, tx, workspaceID, pipelineID); err != nil {
			return err
		}
		deals, err := s.loadRollupDeals(ctx, tx, workspaceID, pipelineID)
		if err != nil {
			return err
		}
		rollup, err := s.buildRollup(ctx, tx, workspaceID, pipelineID, baseCurrency, asOf, deals)
		if err != nil {
			return err
		}
		out = rollup
		return nil
	})
	if err != nil {
		return PipelineRollup{}, err
	}
	return out, nil
}

func (s *RollupStore) loadRollupBaseCurrency(ctx context.Context, tx *sql.Tx, workspaceID string) (string, error) {
	var baseCurrency string
	if err := tx.QueryRowContext(ctx, `
		SELECT base_currency
		FROM workspace
		WHERE id=$1::uuid`,
		workspaceID).Scan(&baseCurrency); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errs.ErrNotFound
		}
		return "", err
	}
	return baseCurrency, nil
}

func (s *RollupStore) ensureRollupPipelineExists(ctx context.Context, tx *sql.Tx, workspaceID, pipelineID string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT true
		FROM pipeline
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
		pipelineID, workspaceID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errs.ErrNotFound
		}
		return err
	}
	return nil
}

func (s *RollupStore) loadRollupDeals(ctx context.Context, tx *sql.Tx, workspaceID, pipelineID string) ([]rollupDealRow, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT d.id, d.stage_id, d.amount_minor, d.currency, s.win_probability, s.position
		FROM deal d
		JOIN stage s
		  ON s.id = d.stage_id
		 AND s.workspace_id = d.workspace_id
		 AND s.archived_at IS NULL
		WHERE d.workspace_id=$1::uuid
		  AND d.pipeline_id=$2::uuid
		  AND d.status='open'
		  AND d.archived_at IS NULL
		ORDER BY s.position, d.id`,
		workspaceID, pipelineID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	deals := make([]rollupDealRow, 0, 16)
	for rows.Next() {
		var row rollupDealRow
		var stagePosition int
		if err := rows.Scan(&row.dealID, &row.stageID, &row.amountMinor, &row.currency, &row.winProbability, &stagePosition); err != nil {
			return nil, err
		}
		deals = append(deals, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return deals, nil
}

func (s *RollupStore) buildRollup(ctx context.Context, tx *sql.Tx, workspaceID, pipelineID, baseCurrency string, asOf time.Time, deals []rollupDealRow) (PipelineRollup, error) {
	out := PipelineRollup{
		PipelineID:   pipelineID,
		BaseCurrency: baseCurrency,
		AsOfDate:     asOf.UTC().Format("2006-01-02"),
		ByStage:      []PipelineRollupStage{},
		Breakdown:    []RollupDealRow{},
	}

	byStage := map[string]*PipelineRollupStage{}
	stageOrder := make([]string, 0, 8)
	for _, row := range deals {
		dealRow := RollupDealRow{DealID: row.dealID, WinProbability: row.winProbability}
		if !row.amountMinor.Valid {
			dealRow.NoAmount = true
		} else {
			baseValue := row.amountMinor.Int64
			if row.currency.Valid && row.currency.String != "" && row.currency.String != baseCurrency {
				rate, err := AsOfFXRate(ctx, tx, workspaceID, row.currency.String, baseCurrency, asOf)
				if err != nil {
					return PipelineRollup{}, err
				}
				baseValue = ConvertToBase(row.amountMinor.Int64, row.currency.String, baseCurrency, rate)
			}
			dealRow.BaseValueMinor = baseValue
			dealRow.WeightedValueMinor = WeightedValue(baseValue, row.winProbability)
		}
		out.Breakdown = append(out.Breakdown, dealRow)

		stageRow := byStage[row.stageID]
		if stageRow == nil {
			stageRow = &PipelineRollupStage{StageID: row.stageID}
			byStage[row.stageID] = stageRow
			stageOrder = append(stageOrder, row.stageID)
		}
		stageRow.UnweightedMinor += dealRow.BaseValueMinor
		stageRow.WeightedMinor += dealRow.WeightedValueMinor
		stageRow.DealCount++
	}

	totals := SumRollup(out.Breakdown)
	out.UnweightedMinor = totals.UnweightedMinor
	out.WeightedMinor = totals.WeightedMinor
	out.ByStage = make([]PipelineRollupStage, 0, len(stageOrder))
	for _, stageID := range stageOrder {
		out.ByStage = append(out.ByStage, *byStage[stageID])
	}
	return out, nil
}
