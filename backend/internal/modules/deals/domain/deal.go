// Package domain contains the deals module's pure domain types.
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// statusOpen is the canonical deal lifecycle value for an open/in-progress deal.
const statusOpen = "open"

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

// Deal is a sales opportunity (data-model §6.3).
type Deal struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	Name              string     `json:"name"`
	AmountMinor       *int64     `json:"amount_minor"`
	Currency          *string    `json:"currency"`
	FxRateToBase      *float64   `json:"fx_rate_to_base"`
	FxRateDate        *time.Time `json:"fx_rate_date"`
	PipelineID        string     `json:"pipeline_id"`
	StageID           string     `json:"stage_id"`
	OrganizationID    *string    `json:"organization_id"`
	OwnerID           *string    `json:"owner_id"`
	PartnerOrgID      *string    `json:"partner_org_id"`
	Status            string     `json:"status"` // open | won | lost
	LostReason        *string    `json:"lost_reason"`
	ExpectedCloseDate *time.Time `json:"expected_close_date"`
	ClosedAt          *time.Time `json:"closed_at"`
	ForecastCategory  *string    `json:"forecast_category"`
	WaitUntil         *time.Time `json:"wait_until"`
	LastActivityAt    *time.Time `json:"last_activity_at"`
	Stalled           bool       `json:"stalled"`
	StageEnteredAt    *time.Time `json:"stage_entered_at"`
	StakeholderCount  int        `json:"stakeholder_count"`
	Version           int64      `json:"version"`
	Source            string     `json:"source"`
	CapturedBy        string     `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// AdvanceInput carries a validated advanceDeal request body.
type AdvanceInput struct {
	ToStageID  string
	Status     string // client-supplied status, "" if the caller omitted it
	LostReason *string
}

// DealListFilter holds optional predicates for ListFiltered. Zero value = no extra filters.
type DealListFilter struct {
	PipelineID       string
	StageID          string
	OwnerID          string
	OrganizationID   string
	Status           string // "" | open | won | lost (validated by the caller)
	Stalled          bool
	ForecastCategory string
	PartnerOrgID     string
	PersonID         string
	Sort             string
}

// NewDeal returns a Deal with a fresh ID, open status, version 1, and copied provenance.
func NewDeal(name, pipelineID, stageID string, p prov.Provenance) Deal {
	return Deal{
		ID: ids.New(), Name: name, PipelineID: pipelineID, StageID: stageID, Status: statusOpen,
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

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
