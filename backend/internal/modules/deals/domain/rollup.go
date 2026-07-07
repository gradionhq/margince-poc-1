// Package domain: DEAL-FORM-2's pure arithmetic for rounding, per-deal FX conversion,
// weighted value calculation, total reconciliation, and rollup wire shapes.
package domain

import "math"

// RoundHalfAwayFromZero rounds x to the nearest integer, with half values rounding away
// from zero.
func RoundHalfAwayFromZero(x float64) int64 {
	if x >= 0 {
		return int64(math.Floor(x + 0.5))
	}
	return int64(math.Ceil(x - 0.5))
}

// ConvertToBase converts a minor-unit amount into base-currency minor units using the
// supplied native-to-base rate. Same-currency values pass through unchanged.
func ConvertToBase(amountMinor int64, currency, baseCurrency string, rate float64) int64 {
	if currency == baseCurrency {
		return amountMinor
	}
	return RoundHalfAwayFromZero(float64(amountMinor) * rate)
}

// WeightedValue computes DEAL-FORM-2's per-deal weighted value from a base-currency
// amount and win probability.
func WeightedValue(baseValueMinor int64, winProbability int) int64 {
	return RoundHalfAwayFromZero(float64(baseValueMinor) * float64(winProbability) / 100)
}

// RollupDealRow is one deal's contribution to a roll-up breakdown.
type RollupDealRow struct {
	DealID             string `json:"deal_id"`
	BaseValueMinor     int64  `json:"base_value"`
	WinProbability     int    `json:"win_probability"`
	WeightedValueMinor int64  `json:"weighted_value"`
	NoAmount           bool   `json:"no_amount,omitempty"`
}

// RollupTotals are the sums of the already-rounded per-deal values.
type RollupTotals struct {
	UnweightedMinor int64
	WeightedMinor   int64
}

// SumRollup adds the displayed per-deal values without re-rounding the totals.
func SumRollup(rows []RollupDealRow) RollupTotals {
	var totals RollupTotals
	for _, row := range rows {
		totals.UnweightedMinor += row.BaseValueMinor
		totals.WeightedMinor += row.WeightedValueMinor
	}
	return totals
}

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
