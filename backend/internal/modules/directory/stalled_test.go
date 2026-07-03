package crmcore

import (
	"testing"
	"time"
)

// fixedClock is TEST-DET-1's pinned test clock — every IsStalled test case
// evaluates idle-duration against this instant, never time.Now().
var fixedClock = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)

func daysBefore(clock time.Time, days int) *time.Time {
	t := clock.Add(-time.Duration(days) * 24 * time.Hour)
	return &t
}

func daysAfter(clock time.Time, days int) *time.Time {
	t := clock.Add(time.Duration(days) * 24 * time.Hour)
	return &t
}

func baseOpenDeal() Deal {
	return Deal{Status: statusOpen, CreatedAt: fixedClock.Add(-365 * 24 * time.Hour)}
}

func TestIsStalled(t *testing.T) {
	cases := []struct {
		name       string
		deal       Deal
		wantStall  bool
		wantReason string
	}{
		{
			// UAT-1 / fixture "asked-to-wait-deal" idle leg: 65d idle, no wait_until -> stalled.
			name: "65d_idle_no_wait_until_stalled",
			deal: func() Deal {
				d := baseOpenDeal()
				d.LastActivityAt = daysBefore(fixedClock, 65)
				return d
			}(),
			wantStall:  true,
			wantReason: StalledReasonNoActivity60Days,
		},
		{
			// UAT-2: same shape, 5d idle -> not stalled.
			name: "5d_idle_not_stalled",
			deal: func() Deal {
				d := baseOpenDeal()
				d.LastActivityAt = daysBefore(fixedClock, 5)
				return d
			}(),
			wantStall: false,
		},
		{
			// UAT-3 / fixture "asked-to-wait-suppressed": 65d idle, wait_until 30d ahead -> suppressed.
			name: "65d_idle_wait_30d_ahead_suppressed",
			deal: func() Deal {
				d := baseOpenDeal()
				d.LastActivityAt = daysBefore(fixedClock, 65)
				d.WaitUntil = daysAfter(fixedClock, 30)
				return d
			}(),
			wantStall: false,
		},
		{
			// UAT-4 / fixture "wait-expired": 65d idle, wait_until yesterday -> stalled again.
			name: "65d_idle_wait_yesterday_expired_stalled",
			deal: func() Deal {
				d := baseOpenDeal()
				d.LastActivityAt = daysBefore(fixedClock, 65)
				d.WaitUntil = daysBefore(fixedClock, 1)
				return d
			}(),
			wantStall:  true,
			wantReason: StalledReasonNoActivity60Days,
		},
		{
			// UAT-5: closed deal, 200d idle, no wait_until -> never stalled.
			name: "closed_deal_never_stalls",
			deal: func() Deal {
				d := baseOpenDeal()
				d.Status = statusLost
				d.LastActivityAt = daysBefore(fixedClock, 200)
				return d
			}(),
			wantStall: false,
		},
		{
			// UAT-6: last_activity_at NULL, created_at 95d before clock -> created_at fallback flags.
			name: "never_active_created_at_fallback_stalled",
			deal: func() Deal {
				return Deal{Status: statusOpen, CreatedAt: fixedClock.Add(-95 * 24 * time.Hour)}
			}(),
			wantStall:  true,
			wantReason: StalledReasonNoActivity60Days,
		},
		{
			// UAT-7 / chapter worked example: last activity 2026-03-01 (~95d idle) -> stalled.
			name: "worked_example_95d_idle_stalled",
			deal: func() Deal {
				d := baseOpenDeal()
				la := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
				d.LastActivityAt = &la
				return d
			}(),
			wantStall:  true,
			wantReason: StalledReasonNoActivity60Days,
		},
		{
			// Worked example continued: same deal + wait_until=2026-09-01 -> suppressed.
			name: "worked_example_wait_until_2026_09_01_suppressed",
			deal: func() Deal {
				d := baseOpenDeal()
				la := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
				wu := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
				d.LastActivityAt = &la
				d.WaitUntil = &wu
				return d
			}(),
			wantStall: false,
		},
		{
			// Worked example: last activity 2026-05-25 (~10d idle) -> not stalled.
			name: "worked_example_10d_idle_not_stalled",
			deal: func() Deal {
				d := baseOpenDeal()
				la := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
				d.LastActivityAt = &la
				return d
			}(),
			wantStall: false,
		},
		{
			// Boundary: wait_until is exactly today (date(now_utc) == wait_until) -> still suppressed
			// (holds THROUGH end of wait day, UTC).
			name: "wait_until_today_still_suppressed",
			deal: func() Deal {
				d := baseOpenDeal()
				d.LastActivityAt = daysBefore(fixedClock, 65)
				wu := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
				d.WaitUntil = &wu
				return d
			}(),
			wantStall: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStall, gotReason := IsStalled(tc.deal, fixedClock)
			if gotStall != tc.wantStall {
				t.Errorf("IsStalled() stalled = %v, want %v", gotStall, tc.wantStall)
			}
			if gotStall && gotReason != tc.wantReason {
				t.Errorf("IsStalled() reason = %q, want %q", gotReason, tc.wantReason)
			}
			if !gotStall && gotReason != "" {
				t.Errorf("IsStalled() reason = %q, want empty when not stalled", gotReason)
			}
		})
	}
}

// TestIsStalled_ThresholdIsAbsoluteDuration_DSTImmune proves the idle check is a
// plain instant subtraction, not calendar-day arithmetic — pin at exactly the
// 60-day boundary from both sides.
func TestIsStalled_ThresholdBoundary(t *testing.T) {
	// Exactly 60 days idle (not > 60) -> not yet stalled ("idle <= threshold: return false").
	atThreshold := baseOpenDeal()
	atThreshold.LastActivityAt = daysBefore(fixedClock, StalledThresholdDays)
	if stalled, _ := IsStalled(atThreshold, fixedClock); stalled {
		t.Error("idle == StalledThresholdDays exactly must NOT be stalled (idle <= threshold)")
	}

	// One hour past the 60-day boundary -> stalled.
	pastThreshold := baseOpenDeal()
	la := fixedClock.Add(-time.Duration(StalledThresholdDays)*24*time.Hour - time.Hour)
	pastThreshold.LastActivityAt = &la
	if stalled, reason := IsStalled(pastThreshold, fixedClock); !stalled || reason != StalledReasonNoActivity60Days {
		t.Errorf("idle just past threshold: stalled=%v reason=%q, want true/%q", stalled, reason, StalledReasonNoActivity60Days)
	}
}
