package app_test

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func confidence(v float64) *float64 { return &v }

func completeFixture() domain.Proposal {
	return domain.Proposal{
		WorkspaceID:  "ws-1",
		ActionType:   "log_link",
		TargetEntity: "activity:1",
		Effect:       []byte(`{}`),
		Evidence:     "call transcript mentions X",
		Confidence:   confidence(0.9),
		Source:       "capture:call:abc",
	}
}

func TestGateProposals_CompleteFixturePasses(t *testing.T) {
	out := app.GateProposals([]domain.Proposal{completeFixture()})
	if len(out) != 1 {
		t.Fatalf("expected 1 surviving proposal, got %d", len(out))
	}
}

func TestGateProposals_DropsEachMissingFieldIndividually(t *testing.T) {
	cases := map[string]func(domain.Proposal) domain.Proposal{
		"missing source":     func(p domain.Proposal) domain.Proposal { p.Source = ""; return p },
		"missing evidence":   func(p domain.Proposal) domain.Proposal { p.Evidence = ""; return p },
		"missing confidence": func(p domain.Proposal) domain.Proposal { p.Confidence = nil; return p },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := mutate(completeFixture())
			out := app.GateProposals([]domain.Proposal{p})
			if len(out) != 0 {
				t.Fatalf("%s: expected proposal dropped, got %d survivors", name, len(out))
			}
		})
	}
}
