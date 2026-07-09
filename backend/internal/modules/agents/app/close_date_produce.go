package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
)

const (
	closeDateEvidenceConfidence = 1.0 // deterministic rule, not a model guess (Global Constraint 1)
	closeDateSource             = "policy:FCAST-FORM-3"
	closeDateEventTopic         = "deal.updated" // OVN-EVT-4, literal
)

// NewCloseDateProduce returns a ports.Produce closure implementing FCAST-FORM-3
// over dealReader's read port. Produce carries no context/tx (ports.Produce's
// own signature, unchanged — Pre-implementation Finding 1); this closure uses
// context.Background() for its own reads. It reads the workspace to sweep off
// view.WorkspaceID — the pass's own read of "the day" already scopes it there,
// so no second workspace parameter is needed.
func NewCloseDateProduce(dealReader ports.DealReader, clock func() time.Time) ports.Produce {
	return func(view domain.AssembledView) ([]domain.Proposal, error) {
		ctx := context.Background()
		now := clock()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		snaps, err := dealReader.ListOpenDeals(ctx, view.WorkspaceID, now)
		if err != nil {
			return nil, err
		}

		velocityCache := map[string]int{}
		out := make([]domain.Proposal, 0, len(snaps))
		for _, s := range snaps {
			flags := ComputeCloseDateFlags(s.Status, s.ExpectedCloseDate, today, s.WinProbability, s.IsStalled)
			if !flags.Any() {
				continue
			}

			velocity, ok := velocityCache[s.PipelineID]
			if !ok {
				median, wonCount, vErr := dealReader.PipelineWonVelocity(ctx, s.WorkspaceID, s.PipelineID)
				if vErr != nil {
					return out, vErr
				}
				velocity = StageVelocityDays(median, wonCount)
				velocityCache[s.PipelineID] = velocity
			}

			proposed := ProposedCloseDate(today, s.RemainingOpenStages, velocity)
			quiet := s.IsStalled
			clearOverdue := flags.Overdue && !flags.Missing && !flags.UnrealisticSoon
			active := !quiet
			inCommit := InForecastCommit(s.ForecastCategory, s.WinProbability)
			lateStage := s.WinProbability >= ForecastBestcaseMinProb
			action := DecideCloseDateAction(quiet, clearOverdue, active, inCommit, lateStage, CloseDateAutoApply)
			out = append(out, closeDateProposals(s, proposed, action)...)
		}
		return out, nil
	}
}

// closeDateBaseEffect builds the effect payload for the two green
// (immediately-applied) action types: if_match carries the snapshot's real
// version, since these are the only write against the deal in this pass
// (Pre-implementation Finding 7).
func closeDateBaseEffect(s ports.DealSnapshot, priorClose *string, newDate string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"deal_id":          s.DealID,
		"workspace_id":     s.WorkspaceID,
		"if_match":         s.Version,
		"new_close_date":   newDate,
		"prior_close_date": priorClose,
		"prior_version":    s.Version,
		"event_topic":      closeDateEventTopic,
	})
	return b
}

// closeDateStagedEffect builds the effect payload for the two yellow
// (decided-later) action types: if_match is always 0 (skip the version
// check) since the paired close-date-provisional-set write has already
// bumped the deal's version by decision time — a captured prior_version
// would be stale (Pre-implementation Finding 7).
func closeDateStagedEffect(s ports.DealSnapshot, priorClose *string, newDate, reason, message string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"deal_id":          s.DealID,
		"workspace_id":     s.WorkspaceID,
		"if_match":         0,
		"new_close_date":   newDate,
		"prior_close_date": priorClose,
		"prior_version":    s.Version,
		"event_topic":      closeDateEventTopic,
		"reason":           reason,
		"message":          message,
	})
	return b
}

// closeDateProposals turns one flagged deal snapshot plus its decided
// CloseDateAction into the one (AUTO_APPLY) or two (PROVISIONAL_CONFIRM/
// DOWNGRADE_AND_REVIEW) proposals FCAST-FORM-3's two-proposal green+yellow
// design requires: the always-green provisional-set placeholder satisfies
// the OVN-AC-1 open-deal invariant regardless of tier, paired with a yellow
// confirm-request/downgrade-review item for human review.
func closeDateProposals(s ports.DealSnapshot, proposed time.Time, action CloseDateAction) []domain.Proposal {
	target := "deal:" + s.DealID
	conf := closeDateEvidenceConfidence
	proposedStr := proposed.Format("2006-01-02")
	var priorClose *string
	if s.ExpectedCloseDate != nil {
		v := s.ExpectedCloseDate.Format("2006-01-02")
		priorClose = &v
	}

	if action == ActionAutoApply {
		return []domain.Proposal{{
			WorkspaceID:  s.WorkspaceID,
			ActionType:   "close-date-auto-apply",
			TargetEntity: target,
			Effect:       closeDateBaseEffect(s, priorClose, proposedStr),
			Evidence:     "clear-overdue, active, non-forecast-bearing deal auto-corrected to " + proposedStr,
			Confidence:   &conf,
			Source:       closeDateSource,
			EventTopic:   closeDateEventTopic,
		}}
	}

	provisional := domain.Proposal{
		WorkspaceID:  s.WorkspaceID,
		ActionType:   "close-date-provisional-set",
		TargetEntity: target,
		Effect:       closeDateBaseEffect(s, priorClose, proposedStr),
		Evidence:     "invariant placeholder: replaced an invalid close date with a provisional " + proposedStr,
		Confidence:   &conf,
		Source:       closeDateSource,
		EventTopic:   closeDateEventTopic,
	}

	if action == ActionDowngradeAndReview {
		return []domain.Proposal{provisional, {
			WorkspaceID:  s.WorkspaceID,
			ActionType:   "close-date-downgrade-review",
			TargetEntity: target,
			Effect:       closeDateStagedEffect(s, priorClose, proposedStr, "quiet", "This deal has gone quiet -- is it still alive? Set a real date or mark lost."),
			Evidence:     "deal has gone quiet (no real activity for the stalled threshold)",
			Confidence:   &conf,
			Source:       closeDateSource,
			EventTopic:   closeDateEventTopic,
		}}
	}

	return []domain.Proposal{provisional, {
		WorkspaceID:  s.WorkspaceID,
		ActionType:   "close-date-confirm-request",
		TargetEntity: target,
		Effect:       closeDateStagedEffect(s, priorClose, proposedStr, "provisional_confirm", "Confirm the real close date for this deal."),
		Evidence:     "forecast-bearing, late-stage, missing, or unrealistic close date needs human confirmation",
		Confidence:   &conf,
		Source:       closeDateSource,
		EventTopic:   closeDateEventTopic,
	}}
}
