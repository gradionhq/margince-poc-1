package crmaudit

import "testing"

func TestSmellRow_ManualPct(t *testing.T) {
	r := SmellRow{Total: 4, Manual: 2}
	if r.Total > 0 {
		r.ManualPct = float64(r.Manual) / float64(r.Total)
	}
	if r.ManualPct != 0.5 {
		t.Fatalf("ManualPct=%v want 0.5", r.ManualPct)
	}
}
