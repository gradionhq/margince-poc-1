package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

const (
	mailActionIntegrityFlag = "integrity_flag"
	mailCheckName           = "proposal_sent_without_mail"
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
	return produceIntegrityFlags(view, "proposal_sent_claim", "outbound_email_trace", mailActionIntegrityFlag, mailCheckName, "no outbound email found for this proposal-sent claim", decodeProposalSentClaim), nil
}

func decodeProposalSentClaim(raw string) (string, *float64, bool) {
	var d proposalSentClaimDetail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return "", nil, false
	}
	if d.Confidence == nil || d.Description == "" {
		return "", nil, false
	}
	return d.Description, d.Confidence, true
}
