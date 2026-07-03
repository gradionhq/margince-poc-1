package crmcore

import (
	"fmt"
	"testing"
	"time"
)

// fixedStrengthClock is TEST-DET-1's pinned test clock for PO-F-3, matching
// the worked example in docs/subsystems/people-and-organizations.md:389-393.
var fixedStrengthClock = time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)

func strengthDaysBefore(clock time.Time, days float64) time.Time {
	return clock.Add(-time.Duration(days * float64(24*time.Hour)))
}

func direction(d string) *string { return &d }

func strengthID(prefix string, days float64) string {
	return fmt.Sprintf("%s-%g", prefix, days)
}

// TestComputeStrength_GoldenExample proves the exact worked example: last
// interaction 5 days ago, 12 interactions in 90d (7 inbound/5 outbound) ->
// recency=0.891, frequency=0.60, reciprocity=0.875, score=47, bucket=moderate.
func TestComputeStrength_GoldenExample(t *testing.T) {
	var activities []StrengthActivity
	// 7 inbound, spread across the 90d window, most recent 5 days ago.
	inboundDays := []float64{5, 10, 18, 25, 40, 55, 70}
	for _, d := range inboundDays {
		activities = append(activities, StrengthActivity{
			ID:         strengthID("a-in", d),
			Kind:       "email",
			OccurredAt: strengthDaysBefore(fixedStrengthClock, d),
			Direction:  direction("inbound"),
		})
	}
	// 5 outbound.
	outboundDays := []float64{8, 20, 35, 50, 65}
	for _, d := range outboundDays {
		activities = append(activities, StrengthActivity{
			ID:         strengthID("a-out", d),
			Kind:       "call",
			OccurredAt: strengthDaysBefore(fixedStrengthClock, d),
			Direction:  direction("outbound"),
		})
	}

	got := ComputeStrength(fixedStrengthClock, activities)

	if got.NoSignalYet || got.NoRecentActivity {
		t.Fatalf("golden case must not be no-signal/no-recent: %+v", got)
	}
	const tol = 0.001
	if diff := got.Recency - 0.891; diff > tol || diff < -tol {
		t.Errorf("Recency = %v, want ~0.891", got.Recency)
	}
	if got.Frequency != 0.60 {
		t.Errorf("Frequency = %v, want 0.60", got.Frequency)
	}
	if diff := got.Reciprocity - 0.875; diff > tol || diff < -tol {
		t.Errorf("Reciprocity = %v, want ~0.875", got.Reciprocity)
	}
	if got.Score != 47 {
		t.Errorf("Score = %d, want 47", got.Score)
	}
	if got.Bucket != StrengthBucketModerate {
		t.Errorf("Bucket = %q, want %q", got.Bucket, StrengthBucketModerate)
	}
	if len(got.ContributingActivities) != 12 {
		t.Errorf("ContributingActivities len = %d, want 12", len(got.ContributingActivities))
	}
}

// TestComputeStrength_NoSignalYet: zero interactions ever -> undefined, never
// a literal 0 score.
func TestComputeStrength_NoSignalYet(t *testing.T) {
	got := ComputeStrength(fixedStrengthClock, nil)
	if !got.NoSignalYet {
		t.Fatal("want NoSignalYet=true for zero interactions ever")
	}
	if got.NoRecentActivity {
		t.Error("NoRecentActivity must not also be set")
	}
}

// TestComputeStrength_NoRecentActivity: interactions exist but none in the
// 90d window -> score=0, bucket=weak, no-recent-activity marker, balance
// never evaluated as 0/0 (this must not panic/NaN).
func TestComputeStrength_NoRecentActivity(t *testing.T) {
	activities := []StrengthActivity{
		{ID: "old-1", Kind: "email", OccurredAt: strengthDaysBefore(fixedStrengthClock, 120), Direction: direction("inbound")},
		{ID: "old-2", Kind: "call", OccurredAt: strengthDaysBefore(fixedStrengthClock, 200), Direction: direction("outbound")},
	}
	got := ComputeStrength(fixedStrengthClock, activities)
	if got.NoSignalYet {
		t.Fatal("interactions exist ever -> NoSignalYet must be false")
	}
	if !got.NoRecentActivity {
		t.Fatal("want NoRecentActivity=true when nothing is within the 90d window")
	}
	if got.Score != 0 {
		t.Errorf("Score = %d, want 0", got.Score)
	}
	if got.Bucket != StrengthBucketWeak {
		t.Errorf("Bucket = %q, want %q", got.Bucket, StrengthBucketWeak)
	}
	if len(got.ContributingActivities) != 0 {
		t.Errorf("ContributingActivities len = %d, want 0 (nothing in window)", len(got.ContributingActivities))
	}
}

// TestComputeStrength_AllOneDirectional: all-inbound within the window ->
// reciprocity floors at exactly 0.25, never lower.
func TestComputeStrength_AllOneDirectional(t *testing.T) {
	activities := []StrengthActivity{
		{ID: "in-1", Kind: "email", OccurredAt: strengthDaysBefore(fixedStrengthClock, 1), Direction: direction("inbound")},
		{ID: "in-2", Kind: "email", OccurredAt: strengthDaysBefore(fixedStrengthClock, 2), Direction: direction("inbound")},
		{ID: "in-3", Kind: "meeting", OccurredAt: strengthDaysBefore(fixedStrengthClock, 3), Direction: direction("inbound")},
	}
	got := ComputeStrength(fixedStrengthClock, activities)
	if got.Reciprocity != RelStrengthReciprocityFloor {
		t.Errorf("Reciprocity = %v, want exactly the floor %v", got.Reciprocity, RelStrengthReciprocityFloor)
	}
}

// TestStrengthBucket proves the PO-PARAM-3 thresholds exactly (boundary values).
func TestStrengthBucket(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{0, StrengthBucketWeak}, {24, StrengthBucketWeak},
		{25, StrengthBucketModerate}, {59, StrengthBucketModerate},
		{60, StrengthBucketStrong}, {100, StrengthBucketStrong},
	}
	for _, tc := range cases {
		if got := StrengthBucket(tc.score); got != tc.want {
			t.Errorf("StrengthBucket(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}
