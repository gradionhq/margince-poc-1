package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// Fact convention for the meeting-without-recap check:
//   - EntityType = "meeting_claim": a meeting was logged. Detail:
//     {"confidence": <float>, "description": "<string>"} - both
//     required.
//   - EntityType = "meeting_recap_trace": corroborating evidence a
//     recap was captured for that meeting. Only its presence for the
//     same EntityID matters.

type meetingClaimDetail struct {
	Confidence  *float64 `json:"confidence"`
	Description string   `json:"description"`
}

// ProduceMeetingRecapFlags emits one "integrity_flag" Proposal per
// well-formed "meeting_claim" Fact lacking a same-EntityID
// "meeting_recap_trace" Fact. Order-preserving. Never errors (see
// producer_integrity_check.go).
func ProduceMeetingRecapFlags(view domain.AssembledView) ([]domain.Proposal, error) {
	recapped := map[string]bool{}
	for _, f := range view.Facts {
		if f.EntityType == "meeting_recap_trace" {
			recapped[f.EntityID] = true
		}
	}

	var out []domain.Proposal
	for _, f := range view.Facts {
		if f.EntityType != "meeting_claim" {
			continue
		}
		detail, ok := decodeMeetingClaim(f.Detail)
		if !ok || recapped[f.EntityID] {
			continue
		}
		effect, _ := json.Marshal(map[string]string{
			"check": "meeting_without_recap",
			"claim": detail.Description,
		})
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "integrity_flag",
			TargetEntity: f.EntityID,
			Effect:       effect,
			Evidence:     "no recap found for this logged meeting",
			Confidence:   detail.Confidence,
			Source:       f.Source,
		})
	}
	return out, nil
}

func decodeMeetingClaim(raw string) (meetingClaimDetail, bool) {
	var d meetingClaimDetail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return meetingClaimDetail{}, false
	}
	if d.Confidence == nil || d.Description == "" {
		return meetingClaimDetail{}, false
	}
	return d, true
}
