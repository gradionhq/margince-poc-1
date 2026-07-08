// Package app contains the agents module's application-service layer: the
// no-guess gate, tier router, stager/apply integration, approval-decided
// executor, and batch assembler.
package app

import "github.com/gradionhq/margince/backend/internal/modules/agents/domain"

// GateProposals drops any proposal missing a resolvable source, a
// non-empty evidence snippet, or a confidence (GATE-AI-1/OVN-AC-1) — the
// pass would rather stay silent than guess. Order-preserving.
func GateProposals(in []domain.Proposal) []domain.Proposal {
	out := make([]domain.Proposal, 0, len(in))
	for _, p := range in {
		if p.Source == "" || p.Evidence == "" || p.Confidence == nil {
			continue
		}
		out = append(out, p)
	}
	return out
}
