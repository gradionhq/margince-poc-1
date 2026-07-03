package crmcore

import "time"

// Stalled-flag tunables (DEAL-PARAM-1/2, deals-and-pipeline.md "Formulas — stalled
// deal rule"). Deliberately named Go source constants, not runtime config, per the
// chapter's explicit "code edit + redeploy to change" note.
const (
	// StalledThresholdDays is the idle threshold for the stalled flag (DEAL-PARAM-1):
	// an absolute duration over UTC instants, not calendar days.
	StalledThresholdDays = 60
	// StalledAskedToWaitDays is the suppression window a dateless capture-pipeline
	// deferral would apply (DEAL-PARAM-2) — consumed by the capture chapter's L2
	// commitment-extraction work, explicitly out of this ticket's scope. Named here
	// because DEAL-PARAM-2 is pinned alongside DEAL-PARAM-1 in the same formula.
	StalledAskedToWaitDays = 90
	// StalledReasonNoActivity60Days is the default stalled reason DEAL-FORM-3
	// produces; richer reasons are the morning-brief chapter's scope.
	StalledReasonNoActivity60Days = "no_activity_60_days"
)

// IsStalled implements DEAL-FORM-3 exactly: a deterministic, pure function over the
// deal's idle duration and an optional customer-wait suppression window. now is the
// caller's clock (UTC) — production passes time.Now().UTC(), tests pin a fixed
// instant (TEST-DET-1). No DB/context access: every input is an already-loaded Deal
// field.
//
//	is_stalled(deal, now_utc):
//	    if deal.status != 'open':            return false      # closed deals never stall
//	    base = deal.last_activity_at if not NULL else deal.created_at
//	    idle = now_utc - base                                  # absolute duration, DST-immune
//	    if idle <= StalledThresholdDays * 24h:  return false
//	    if deal.wait_until is not NULL
//	       and date(now_utc) <= deal.wait_until:                # holds through end of wait day, UTC
//	           return false                                     # suppressed
//	    return true                                              # reason: no_activity_60_days
func IsStalled(d Deal, now time.Time) (stalled bool, reason string) {
	if d.Status != statusOpen {
		return false, ""
	}

	base := d.CreatedAt
	if d.LastActivityAt != nil {
		base = *d.LastActivityAt
	}

	idle := now.Sub(base)
	if idle <= StalledThresholdDays*24*time.Hour {
		return false, ""
	}

	if d.WaitUntil != nil {
		// "holds through the end of the wait day, UTC": suppression is active while
		// now's UTC calendar date is on-or-before wait_until's UTC calendar date —
		// truncate both to a UTC-midnight instant so a same-day wait_until at any
		// clock time still suppresses through 23:59:59 UTC of that day.
		nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		waitDate := time.Date(d.WaitUntil.Year(), d.WaitUntil.Month(), d.WaitUntil.Day(), 0, 0, 0, 0, time.UTC)
		if !nowDate.After(waitDate) {
			return false, ""
		}
	}

	return true, StalledReasonNoActivity60Days
}
