package app

import "time"

const (
	CloseDateUnrealisticSoonDays = 7
	CloseDateStageDays           = 14
	CloseDateMinHistory          = 20
	CloseDateAutoApply           = true

	ForecastCommitMinProb   = 90
	ForecastBestcaseMinProb = 50
)

const StalledThresholdDays = 60

type CloseDateFlags struct {
	Overdue          bool
	Missing          bool
	UnrealisticSoon  bool
	UnrealisticStale bool
}

func (f CloseDateFlags) Any() bool {
	return f.Overdue || f.Missing || f.UnrealisticSoon || f.UnrealisticStale
}

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

func StageVelocityDays(observedMedianDays, wonDealCount int) int {
	if wonDealCount >= CloseDateMinHistory {
		return observedMedianDays
	}
	return CloseDateStageDays
}

func ProposedCloseDate(today time.Time, remainingOpenStages, stageVelocityDays int) time.Time {
	stages := remainingOpenStages
	if stages < 1 {
		stages = 1
	}
	return today.AddDate(0, 0, stages*stageVelocityDays)
}

func InForecastCommit(forecastCategory *string, winProbability int) bool {
	if forecastCategory != nil {
		return *forecastCategory == "commit" || *forecastCategory == "best_case"
	}
	return winProbability >= ForecastCommitMinProb
}

type CloseDateAction string

const (
	ActionAutoApply          CloseDateAction = "AUTO_APPLY"
	ActionProvisionalConfirm CloseDateAction = "PROVISIONAL_CONFIRM"
	ActionDowngradeAndReview CloseDateAction = "DOWNGRADE_AND_REVIEW"
)

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
