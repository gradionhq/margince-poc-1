package strength

import (
	"math"
	"sort"
	"time"
)

// PO-F-3 tunables (docs/subsystems/people-and-organizations.md "Formulas §4",
// #parameters RELSTRENGTH_*): the recency half-life, frequency saturation point,
// reciprocity floor, and lookback window used by ComputeStrength.
const (
	RelStrengthHalfLifeDays     = 30.0
	RelStrengthFreqSaturation   = 20.0
	RelStrengthReciprocityFloor = 0.25
	RelStrengthWindowDays       = 90
)

// PO-PARAM-3 buckets: the named strength tiers a computed score is classified
// into for display.
const (
	StrengthBucketWeak     = "weak"
	StrengthBucketModerate = "moderate"
	StrengthBucketStrong   = "strong"
)

// StrengthActivity is the minimal shape ComputeStrength needs from a live
// email/call/meeting activity linked to a person.
type StrengthActivity struct {
	ID         string
	Kind       string
	Subject    *string
	OccurredAt time.Time
	Direction  *string // inbound | outbound | nil
}

// StrengthResult is PO-F-3's output for one person.
type StrengthResult struct {
	Score                  int
	Bucket                 string
	Recency                float64
	Frequency              float64
	Reciprocity            float64
	NoSignalYet            bool
	NoRecentActivity       bool
	ContributingActivities []StrengthActivity
}

// ComputeStrength implements PO-F-3 exactly. now is the caller's clock (UTC);
// production passes time.Now().UTC(), tests pin a fixed instant (TEST-DET-1).
// Pure function — no DB/context access.
func ComputeStrength(now time.Time, activities []StrengthActivity) StrengthResult {
	if len(activities) == 0 {
		return StrengthResult{
			Bucket:      StrengthBucketWeak,
			NoSignalYet: true,
		}
	}

	windowStart := now.Add(-time.Duration(RelStrengthWindowDays) * 24 * time.Hour)
	windowed := make([]StrengthActivity, 0, len(activities))
	var lastInteraction time.Time
	for i, activity := range activities {
		if i == 0 || activity.OccurredAt.After(lastInteraction) {
			lastInteraction = activity.OccurredAt
		}
		if !activity.OccurredAt.Before(windowStart) {
			windowed = append(windowed, activity)
		}
	}

	if len(windowed) == 0 {
		return StrengthResult{
			Score:            0,
			Bucket:           StrengthBucketWeak,
			NoRecentActivity: true,
		}
	}

	sort.Slice(windowed, func(i, j int) bool {
		if windowed[i].OccurredAt.Equal(windowed[j].OccurredAt) {
			return windowed[i].ID < windowed[j].ID
		}
		return windowed[i].OccurredAt.After(windowed[j].OccurredAt)
	})

	recency := recencyScore(now, lastInteraction)
	frequency := math.Min(1.0, float64(len(windowed))/RelStrengthFreqSaturation)
	inbound, outbound := directionalCounts(windowed)
	reciprocity := reciprocityScore(inbound, outbound)

	score := int(math.Round(100 * recency * frequency * reciprocity))
	return StrengthResult{
		Score:                  score,
		Bucket:                 StrengthBucket(score),
		Recency:                recency,
		Frequency:              frequency,
		Reciprocity:            reciprocity,
		ContributingActivities: windowed,
	}
}

// StrengthBucket maps a 0-100 score to its PO-PARAM-3 display bucket.
func StrengthBucket(score int) string {
	switch {
	case score < 25:
		return StrengthBucketWeak
	case score < 60:
		return StrengthBucketModerate
	default:
		return StrengthBucketStrong
	}
}

func recencyScore(now, lastInteraction time.Time) float64 {
	daysSince := now.Sub(lastInteraction).Hours() / 24
	if daysSince < 0 {
		daysSince = 0
	}
	return math.Pow(2, -daysSince/RelStrengthHalfLifeDays)
}

func directionalCounts(activities []StrengthActivity) (inbound, outbound float64) {
	for _, activity := range activities {
		if activity.Direction == nil {
			continue
		}
		switch *activity.Direction {
		case "inbound":
			inbound++
		case "outbound":
			outbound++
		}
	}
	return inbound, outbound
}

func reciprocityScore(inbound, outbound float64) float64 {
	total := inbound + outbound
	if total == 0 {
		return 0
	}
	balance := 1 - math.Abs(inbound-outbound)/total
	return RelStrengthReciprocityFloor + (1-RelStrengthReciprocityFloor)*balance
}
