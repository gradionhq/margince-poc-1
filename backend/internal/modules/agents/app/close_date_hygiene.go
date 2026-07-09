package app

import "time"

// Close-date hygiene tunables (FCAST-PARAM-7..10 / OVN-PARAM-1..4) — named Go
// source constants, no runtime tuning surface (P1). This ticket originates
// them: no forecasting module exists yet anywhere in backend/ (confirmed by
// grep) — these are the first definitions.
const (
	CloseDateUnrealisticSoonDays = 7    // FCAST-PARAM-7
	CloseDateStageDays           = 14   // FCAST-PARAM-8 fallback velocity
	CloseDateMinHistory          = 20   // FCAST-PARAM-9 won-deal floor
	CloseDateAutoApply           = true // FCAST-PARAM-10 master 🟢 switch

	// Forecast-category default floors (FCAST-PARAM-1/2) — this ticket
	// borrows only the narrow in_forecast_commit predicate, never the full
	// category roll-up (out of scope).
	ForecastCommitMinProb   = 90
	ForecastBestcaseMinProb = 50 // also the FCAST-FORM-3 late_stage threshold
)

// StalledThresholdDays mirrors deals/domain.StalledThresholdDays (DEAL-PARAM-1)
// — duplicated as a local named constant rather than importing the deals
// module from this pure, dependency-free file (Pre-implementation Finding 6);
// the agents/adapters.SQLDealReader is the one place that actually calls the
// real deals.IsStalled/StalledThresholdDays.
const StalledThresholdDays = 60

// CloseDateFlags are the four deterministic FCAST-FORM-3 flags, computed only
// for status='open' deals — a won/lost deal is never flagged.
type CloseDateFlags struct {
	Overdue          bool
	Missing          bool
	UnrealisticSoon  bool
	UnrealisticStale bool
}

// Any reports whether the deal is flagged at all (only a flagged deal gets a
// close-date-hygiene proposal).
func (f CloseDateFlags) Any() bool {
	return f.Overdue || f.Missing || f.UnrealisticSoon || f.UnrealisticStale
}

// ComputeCloseDateFlags implements FCAST-FORM-3's flag block. isStalled must
// already reflect deal.WaitUntil suppression (the caller passes
// deals/domain.IsStalled's own result — never reimplemented here), which is
// how a paused deal's unrealistic_stale flag is correctly suppressed without
// any extra logic in this function.
//
// unrealistic_soon is bounded on BOTH sides (today <= date <= today+N), not
// just above — an already-overdue date never trips it, regardless of win
// probability. The corpus formula's plain "<=" reads as unbounded below at
// first glance, but forecasting.md's own worked example calls a
// 12-days-overdue, 20%-win-probability deal clear_overdue (which requires
// overdue AND NOT unrealistic_soon) — if unrealistic_soon fired on any
// already-past date under the 40% floor, clear_overdue would be unreachable
// for every such deal, contradicting that example. unrealistic_soon
// describes a future date that is unrealistically imminent; an
// already-passed date is simply overdue, a distinct flag.
func ComputeCloseDateFlags(status string, expectedCloseDate *time.Time, today time.Time, winProbability int, isStalled bool) CloseDateFlags {
	if status != "open" {
		return CloseDateFlags{}
	}
	if expectedCloseDate == nil {
		return CloseDateFlags{Missing: true}
	}

	overdue := expectedCloseDate.Before(today)
	untilSoon := today.AddDate(0, 0, CloseDateUnrealisticSoonDays)
	unrealisticSoon := !expectedCloseDate.Before(today) && !expectedCloseDate.After(untilSoon) && winProbability < 40
	unrealisticStale := isStalled && !expectedCloseDate.After(today.AddDate(0, 0, StalledThresholdDays))

	return CloseDateFlags{
		Overdue:          overdue,
		UnrealisticSoon:  unrealisticSoon,
		UnrealisticStale: unrealisticStale,
	}
}

// StageVelocityDays picks the workspace-observed median days-per-stage over
// won deals when there's enough history, else the opinionated fallback
// (FCAST-PARAM-8/9).
func StageVelocityDays(observedMedianDays, wonDealCount int) int {
	if wonDealCount >= CloseDateMinHistory {
		return observedMedianDays
	}
	return CloseDateStageDays
}

// ProposedCloseDate computes the replacement date: today plus at least one
// remaining stage times the velocity.
func ProposedCloseDate(today time.Time, remainingOpenStages, stageVelocityDays int) time.Time {
	stages := remainingOpenStages
	if stages < 1 {
		stages = 1
	}
	return today.AddDate(0, 0, stages*stageVelocityDays)
}

// InForecastCommit implements the narrow predicate this ticket borrows from
// the (unbuilt) forecast-category roll-up: a rep-set category always wins;
// only when unset does the probability default apply. Does NOT build the
// roll-up, coverage, or accuracy machinery (out of scope).
func InForecastCommit(forecastCategory *string, winProbability int) bool {
	if forecastCategory != nil {
		return *forecastCategory == "commit" || *forecastCategory == "best_case"
	}
	return winProbability >= ForecastBestcaseMinProb
}

// CloseDateAction is the FCAST-FORM-3 risk-tier decision.
type CloseDateAction string

// Close-date hygiene risk tiers (FCAST-FORM-3).
const (
	ActionAutoApply          CloseDateAction = "AUTO_APPLY"
	ActionProvisionalConfirm CloseDateAction = "PROVISIONAL_CONFIRM"
	ActionDowngradeAndReview CloseDateAction = "DOWNGRADE_AND_REVIEW"
)

// DecideCloseDateAction implements the DECISIONS A6 hybrid policy exactly.
// autoApplyEnabled is threaded as an explicit parameter (never a package
// var) so OVN-PARAM-4's own test can flip it directly, with zero shared
// mutable state (Global Constraint 2).
func DecideCloseDateAction(quiet, clearOverdue, active, inForecastCommit, lateStage, autoApplyEnabled bool) CloseDateAction {
	switch {
	case quiet:
		return ActionDowngradeAndReview
	case autoApplyEnabled && clearOverdue && active && !inForecastCommit && !lateStage:
		return ActionAutoApply
	default:
		return ActionProvisionalConfirm
	}
}
