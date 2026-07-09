package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

const (
	meetingActionIntegrityFlag = "integrity_flag"
	meetingCheckName           = "meeting_without_recap"
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
	return produceIntegrityFlags(view, "meeting_claim", "meeting_recap_trace", meetingActionIntegrityFlag, meetingCheckName, "no recap found for this logged meeting", decodeMeetingClaim), nil
}

func decodeMeetingClaim(raw string) (string, *float64, bool) {
	var d meetingClaimDetail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return "", nil, false
	}
	if d.Confidence == nil || d.Description == "" {
		return "", nil, false
	}
	return d.Description, d.Confidence, true
}
