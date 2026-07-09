package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
)

const closeDateEvidenceConfidence = 1.0 // deterministic rule, not a model guess
const closeDateSource = "policy:FCAST-FORM-3"
const closeDateEventTopic = "deal.updated"

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

func closeDateProposals(s ports.DealSnapshot, proposed time.Time, action CloseDateAction) []domain.Proposal {
	target := "deal:" + s.DealID
	conf := closeDateEvidenceConfidence
	proposedStr := proposed.Format("2006-01-02")
	var priorClose *string
	if s.ExpectedCloseDate != nil {
		v := s.ExpectedCloseDate.Format("2006-01-02")
		priorClose = &v
	}

	baseEffect := func(newDate string) json.RawMessage {
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
	stagedEffect := func(newDate, reason, message string) json.RawMessage {
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

	if action == ActionAutoApply {
		return []domain.Proposal{{
			WorkspaceID:  s.WorkspaceID,
			ActionType:   "close-date-auto-apply",
			TargetEntity: target,
			Effect:       baseEffect(proposedStr),
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
		Effect:       baseEffect(proposedStr),
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
			Effect:       stagedEffect(proposedStr, "quiet", "This deal has gone quiet -- is it still alive? Set a real date or mark lost."),
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
		Effect:       stagedEffect(proposedStr, "provisional_confirm", "Confirm the real close date for this deal."),
		Evidence:     "forecast-bearing, late-stage, missing, or unrealistic close date needs human confirmation",
		Confidence:   &conf,
		Source:       closeDateSource,
		EventTopic:   closeDateEventTopic,
	}}
}
