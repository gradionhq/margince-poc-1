// IntegrityCheckProduce wires ONA-T04's four deterministic cross-checks into
// one ports.Produce - the data-vs-claims integrity pass. Each check
// (producer_integrity_call.go, producer_integrity_mail.go,
// producer_integrity_meeting.go, producer_integrity_stage.go) is pure,
// structural fact-correlation over an already-assembled view - never model
// judgment, never an LLM call. Every check independently enforces the no-guess
// gate on its OWN claim fact (a malformed/incomplete claim is skipped, never
// flagged "by default") before the shared GateProposals (gate.go) ever runs a
// second time over the combined output.
package app

import (
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
)

// IntegrityCheckProduce concatenates all four checks' output, order-preserving
// (call, mail, meeting, stage). Always returns a nil error - every check
// already treats a malformed claim as "skip", not a producer-level failure, so
// there is never a partial-failure signal to surface here (unlike a live-source
// producer that might fail to fetch a remote signal mid-run).
func IntegrityCheckProduce(view domain.AssembledView) ([]domain.Proposal, error) {
	var out []domain.Proposal
	for _, produce := range []func(domain.AssembledView) ([]domain.Proposal, error){
		ProduceUntracedCallFlags,
		ProduceProposalSentFlags,
		ProduceMeetingRecapFlags,
		ProduceStageUnsupportedFlags,
	} {
		proposals, _ := produce(view)
		out = append(out, proposals...)
	}
	return out, nil
}

// var _ proves IntegrityCheckProduce satisfies ports.Produce's exact shape.
var _ ports.Produce = IntegrityCheckProduce
