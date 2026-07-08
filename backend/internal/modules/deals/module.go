// Package deals is the deals/pipeline domain module: pipeline, stage (T10).
// This module.go re-exports all public types and functions from the
// domain/ and adapters/ subdirectories so external callers see an
// unchanged API (WS-E-a structural migration).
package deals

import (
	"context"
	"database/sql"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// Pipeline is a named sales pipeline.
type Pipeline = domain.Pipeline

// Stage is one step in a Pipeline.
type Stage = domain.Stage

// Tier classifies a stage-transition's approval requirement level.
type Tier = domain.Tier

// Tier values classify the approval requirement for a deal-stage transition.
const (
	TierGreen  = domain.TierGreen
	TierYellow = domain.TierYellow
)

// RollupDealRow is one deal's contribution to a roll-up breakdown.
type RollupDealRow = domain.RollupDealRow

// RollupTotals are the sums of the already-rounded per-deal values.
type RollupTotals = domain.RollupTotals

// PipelineRollup is the wire shape of crm.yaml's PipelineRollup schema.
type PipelineRollup = domain.PipelineRollup

// PipelineRollupStage is the wire shape of crm.yaml's PipelineRollupStage schema.
type PipelineRollupStage = domain.PipelineRollupStage

// ---------------------------------------------------------------------------
// Domain function wrappers
// ---------------------------------------------------------------------------

// ResolveTier returns the transition's tier from its FROM and TO stage semantics.
func ResolveTier(fromSemantic, toSemantic string) Tier {
	return domain.ResolveTier(fromSemantic, toSemantic)
}

// ResolveDynamicTier adapts ResolveTier to the toolgate.RegisterResolver arg-map shape.
func ResolveDynamicTier(args map[string]any) mcp.RiskTier {
	return domain.ResolveDynamicTier(args)
}

// RoundHalfAwayFromZero rounds x to the nearest integer, half away from zero.
func RoundHalfAwayFromZero(x float64) int64 {
	return domain.RoundHalfAwayFromZero(x)
}

// ConvertToBase converts a minor-unit amount into base-currency minor units.
func ConvertToBase(amountMinor int64, currency, baseCurrency string, rate float64) int64 {
	return domain.ConvertToBase(amountMinor, currency, baseCurrency, rate)
}

// WeightedValue computes the per-deal weighted value from a base-currency amount and win probability.
func WeightedValue(baseValueMinor int64, winProbability int) int64 {
	return domain.WeightedValue(baseValueMinor, winProbability)
}

// SumRollup adds the displayed per-deal values without re-rounding the totals.
func SumRollup(rows []RollupDealRow) RollupTotals {
	return domain.SumRollup(rows)
}

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// PipelineStore manages pipeline rows.
type PipelineStore = adapters.PipelineStore

// StageStore manages stage rows.
type StageStore = adapters.StageStore

// RollupStore computes GET /pipelines/{id}/rollup over live open deals.
type RollupStore = adapters.RollupStore

// FXRateUnavailableError signals that no stored fx_rate row satisfies the as-of lookup.
type FXRateUnavailableError = adapters.FXRateUnavailableError

// ---------------------------------------------------------------------------
// Adapter constructor wrappers
// ---------------------------------------------------------------------------

// NewPipelineStore returns a PipelineStore backed by db.
func NewPipelineStore(db *sql.DB) *PipelineStore {
	return adapters.NewPipelineStore(db)
}

// NewStageStore returns a StageStore backed by db.
func NewStageStore(db *sql.DB) *StageStore {
	return adapters.NewStageStore(db)
}

// NewRollupStore returns a RollupStore backed by db.
func NewRollupStore(db *sql.DB) *RollupStore {
	return adapters.NewRollupStore(db)
}

// AsOfFXRate returns the most recent fx_rate.rate for fromCurrency->toCurrency with rate_date <= asOf.
func AsOfFXRate(ctx context.Context, tx *sql.Tx, workspaceID, fromCurrency, toCurrency string, asOf time.Time) (float64, error) {
	return adapters.AsOfFXRate(ctx, tx, workspaceID, fromCurrency, toCurrency, asOf)
}

// ---------------------------------------------------------------------------
// Deal domain type aliases
// ---------------------------------------------------------------------------

// Deal is a sales opportunity (data-model §6.3).
type Deal = domain.Deal

// AdvanceInput carries a validated advanceDeal request body.
type AdvanceInput = domain.AdvanceInput

// DealListFilter holds optional predicates for ListFiltered.
type DealListFilter = domain.DealListFilter

// ---------------------------------------------------------------------------
// Deal stalled-flag constants (DEAL-PARAM-1/2)
// ---------------------------------------------------------------------------

const (
	// StalledThresholdDays is the idle threshold for the stalled flag (DEAL-PARAM-1).
	StalledThresholdDays = domain.StalledThresholdDays
	// StalledAskedToWaitDays is the suppression window (DEAL-PARAM-2).
	StalledAskedToWaitDays = domain.StalledAskedToWaitDays
	// StalledReasonNoActivity60Days is the default stalled reason DEAL-FORM-3 produces.
	StalledReasonNoActivity60Days = domain.StalledReasonNoActivity60Days
)

// ---------------------------------------------------------------------------
// Deal domain function wrappers
// ---------------------------------------------------------------------------

// NewDeal returns a Deal with a fresh ID, open status, version 1, and copied provenance.
func NewDeal(name, pipelineID, stageID string, p prov.Provenance) Deal {
	return domain.NewDeal(name, pipelineID, stageID, p)
}

// IsStalled implements DEAL-FORM-3: returns whether a deal is stalled and its reason.
func IsStalled(d Deal, now time.Time) (bool, string) {
	return domain.IsStalled(d, now)
}

// ---------------------------------------------------------------------------
// DealStore adapter type alias and constructor
// ---------------------------------------------------------------------------

// DealStore manages deal rows, including stage transitions and FX freeze.
type DealStore = adapters.DealStore

// NewDealStore returns a DealStore backed by db.
func NewDealStore(db *sql.DB) *DealStore {
	return adapters.NewDealStore(db)
}
