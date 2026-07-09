package adapters

import (
	"math"
	"testing"
	"time"
)

func TestPacePct(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	midpoint := start.Add(end.Sub(start) / 2)
	oneDayBeforeEnd := end.Add(-24 * time.Hour)
	expectedOneDayBefore := float64(oneDayBeforeEnd.Sub(start).Seconds()) / float64(end.Sub(start).Seconds()) * 100

	tests := []struct {
		name  string
		today time.Time
		want  float64
	}{
		{"before periodStart", start.Add(-24 * time.Hour), 0},
		{"at periodEnd", end, 100},
		{"after periodEnd", end.Add(24 * time.Hour), 100},
		{"midpoint", midpoint, 50},
		{"one day before periodEnd", oneDayBeforeEnd, expectedOneDayBefore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pacePct(start, end, tt.today)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("pacePct = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAttainmentBand(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{99.9, "accent"},
		{100.0, "met"},
		{60.0, "accent"},
		{59.9, "behind"},
		{0, "behind"},
		{150, "met"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := attainmentBand(tt.pct); got != tt.want {
				t.Errorf("attainmentBand(%v) = %q, want %q", tt.pct, got, tt.want)
			}
		})
	}
}
