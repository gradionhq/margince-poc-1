package deals

import "testing"

func TestRoundHalfAwayFromZero_Boundary(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{
		{12345.5, 12346},
		{12345.4, 12345},
		{12345.6, 12346},
		{-12345.5, -12346},
		{0, 0},
	}
	for _, c := range cases {
		if got := RoundHalfAwayFromZero(c.in); got != c.want {
			t.Errorf("RoundHalfAwayFromZero(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestWeightedValue(t *testing.T) {
	if got := WeightedValue(10_000_000, 60); got != 6_000_000 {
		t.Errorf("WeightedValue(10_000_000, 60) = %d, want 6_000_000", got)
	}
}

func TestConvertToBase_SameCurrencyPassthrough(t *testing.T) {
	if got := ConvertToBase(10_000_000, "EUR", "EUR", 1.0); got != 10_000_000 {
		t.Errorf("same-currency ConvertToBase = %d, want 10_000_000 unchanged", got)
	}
}

func TestConvertToBase_CrossCurrencyRounds(t *testing.T) {
	if got := ConvertToBase(5_000_000, "USD", "EUR", 0.92); got != 4_600_000 {
		t.Errorf("ConvertToBase(5_000_000, USD, EUR, 0.92) = %d, want 4_600_000", got)
	}
}

func TestWorkedExample(t *testing.T) {
	baseA := ConvertToBase(10_000_000, "EUR", "EUR", 1.0)
	weightedA := WeightedValue(baseA, 60)
	baseB := ConvertToBase(5_000_000, "USD", "EUR", 0.92)
	weightedB := WeightedValue(baseB, 80)

	rows := []RollupDealRow{
		{DealID: "A", BaseValueMinor: baseA, WinProbability: 60, WeightedValueMinor: weightedA},
		{DealID: "B", BaseValueMinor: baseB, WinProbability: 80, WeightedValueMinor: weightedB},
	}
	totals := SumRollup(rows)
	if totals.UnweightedMinor != 14_600_000 {
		t.Errorf("unweighted = %d, want 14_600_000 (€146,000)", totals.UnweightedMinor)
	}
	if totals.WeightedMinor != 9_680_000 {
		t.Errorf("weighted = %d, want 9_680_000 (€96,800)", totals.WeightedMinor)
	}

	var sumUnweighted, sumWeighted int64
	for _, r := range rows {
		sumUnweighted += r.BaseValueMinor
		sumWeighted += r.WeightedValueMinor
	}
	if sumUnweighted != totals.UnweightedMinor || sumWeighted != totals.WeightedMinor {
		t.Fatal("totals must equal the sum of the displayed per-deal values exactly")
	}
}

func TestNoAmountDeal_ContributesZeroWithMarker(t *testing.T) {
	row := RollupDealRow{DealID: "C", BaseValueMinor: 0, WinProbability: 60, WeightedValueMinor: 0, NoAmount: true}
	if !row.NoAmount {
		t.Fatal("no-amount deal must set NoAmount=true")
	}
	totals := SumRollup([]RollupDealRow{row})
	if totals.UnweightedMinor != 0 || totals.WeightedMinor != 0 {
		t.Fatalf("no-amount deal must contribute 0 to both totals, got %+v", totals)
	}
}
