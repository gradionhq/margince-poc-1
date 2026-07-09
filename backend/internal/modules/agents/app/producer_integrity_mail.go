package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// Fact convention for the proposal-sent-without-mail check:
//   - EntityType = "proposal_sent_claim": a deal claims stage "proposal
//     sent" was reached. Detail: {"confidence": <float>,
//     "description": "<string>"} - both required.
//   - EntityType = "outbound_email_trace": corroborating evidence an
//     outbound email was actually sent. Only its presence for the same
//     EntityID matters.

type proposalSentClaimDetail struct {
	Confidence  *float64 `json:"confidence"`
	Description string   `json:"description"`
}

// ProduceProposalSentFlags emits one "integrity_flag" Proposal per
// well-formed "proposal_sent_claim" Fact lacking a same-EntityID
// "outbound_email_trace" Fact. Order-preserving. Never errors (see
// producer_integrity_check.go).
func ProduceProposalSentFlags(view domain.AssembledView) ([]domain.Proposal, error) {
	mailed := map[string]bool{}
	for _, f := range view.Facts {
		if f.EntityType == "outbound_email_trace" {
			mailed[f.EntityID] = true
		}
	}

	var out []domain.Proposal
	for _, f := range view.Facts {
		if f.EntityType != "proposal_sent_claim" {
			continue
		}
		detail, ok := decodeProposalSentClaim(f.Detail)
		if !ok || mailed[f.EntityID] {
			continue
		}
		effect, _ := json.Marshal(map[string]string{
			"check": "proposal_sent_without_mail",
			"claim": detail.Description,
		})
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "integrity_flag",
			TargetEntity: f.EntityID,
			Effect:       effect,
			Evidence:     "no outbound email found for this proposal-sent claim",
			Confidence:   detail.Confidence,
			Source:       f.Source,
		})
	}
	return out, nil
}

func decodeProposalSentClaim(raw string) (proposalSentClaimDetail, bool) {
	var d proposalSentClaimDetail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return proposalSentClaimDetail{}, false
	}
	if d.Confidence == nil || d.Description == "" {
		return proposalSentClaimDetail{}, false
	}
	return d, true
}
