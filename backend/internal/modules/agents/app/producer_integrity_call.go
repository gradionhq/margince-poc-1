// Package app contains the overnight-agent pass wiring and producer logic.
package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

const (
	callActionIntegrityFlag = "integrity_flag"
	callCheckName           = "untraced_call"
)

// Fact convention for the untraced-call check (see this plan's/spec's Fact
// convention table - repeated here so this file is self-contained):
//   - EntityType = "call_claim": a call was logged. Detail is JSON text
//     {"confidence": <float>, "description": "<string>"} - both required;
//     a claim missing either, or with unparseable Detail, is skipped
//     entirely (no-guess applies to the claim itself, not just the
//     corroboration search).
//   - EntityType = "call_trace": corroborating evidence a call actually
//     happened (a calendar entry or a transcript trace). Only its
//     presence for the same EntityID matters - this check never
//     inspects its Detail.

// callClaimDetail is the "call_claim" Fact.Detail JSON shape.
type callClaimDetail struct {
	Confidence  *float64 `json:"confidence"`
	Description string   `json:"description"`
}

// ProduceUntracedCallFlags emits one "integrity_flag" Proposal per
// well-formed "call_claim" Fact lacking a same-EntityID "call_trace"
// Fact. Order-preserving over view.Facts. Never errors - a malformed
// claim is skipped, not an error (see producer_integrity_check.go).
func ProduceUntracedCallFlags(view domain.AssembledView) ([]domain.Proposal, error) {
	return produceIntegrityFlags(view, "call_claim", "call_trace", callActionIntegrityFlag, callCheckName, "no calendar entry or transcript trace found for this call", decodeCallClaim), nil
}

// decodeCallClaim parses a "call_claim" Fact.Detail, returning ok=false
// for unparseable JSON or a missing required field - the single no-guess
// check point for this claim type.
func decodeCallClaim(raw string) (string, *float64, bool) {
	var d callClaimDetail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return "", nil, false
	}
	if d.Confidence == nil || d.Description == "" {
		return "", nil, false
	}
	return d.Description, d.Confidence, true
}
