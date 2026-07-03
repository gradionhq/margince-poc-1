// Package deals: DEAL-FORM-2's pure arithmetic for rounding, per-deal FX conversion,
// weighted value calculation, and total reconciliation.
package deals

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
