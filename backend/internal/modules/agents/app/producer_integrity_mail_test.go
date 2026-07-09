package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func TestProduceProposalSentFlags_MatchingMailYieldsZeroFlags(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		{EntityType: "proposal_sent_claim", EntityID: "deal:1", Detail: `{"confidence":0.85,"description":"proposal sent to buyer"}`, Source: "capture:stage:1"},
		{EntityType: "outbound_email_trace", EntityID: "deal:1", Source: "capture:email:1"},
	}}
	out, err := app.ProduceProposalSentFlags(view)
	if err != nil {
		t.Fatalf("ProduceProposalSentFlags: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 flags for a corroborated proposal-sent claim, got %d: %+v", len(out), out)
	}
}

func TestProduceProposalSentFlags_NoMailYieldsOneFlag(t *testing.T) {
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		{EntityType: "proposal_sent_claim", EntityID: "deal:2", Detail: `{"confidence":0.6,"description":"proposal sent to buyer"}`, Source: "capture:stage:2"},
	}}
	out, err := app.ProduceProposalSentFlags(view)
	if err != nil {
		t.Fatalf("ProduceProposalSentFlags: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 flag for an unmailed proposal-sent claim, got %d", len(out))
	}
	p := out[0]
	if p.ActionType != "integrity_flag" || p.TargetEntity != "deal:2" || p.Evidence == "" {
		t.Fatalf("unexpected proposal shape: %+v", p)
	}
	if p.Confidence == nil || *p.Confidence != 0.6 {
		t.Errorf("Confidence = %v, want 0.6", p.Confidence)
	}
	if p.Source != "capture:stage:2" {
		t.Errorf("Source = %q, want capture:stage:2", p.Source)
	}
	var effect map[string]string
	if err := json.Unmarshal(p.Effect, &effect); err != nil {
		t.Fatalf("Effect unmarshal: %v", err)
	}
	if effect["check"] != "proposal_sent_without_mail" || effect["claim"] != "proposal sent to buyer" {
		t.Errorf("Effect = %+v", effect)
	}
}

func TestProduceProposalSentFlags_MalformedClaimYieldsZeroFlags(t *testing.T) {
	cases := map[string]string{
		"not json":            `not-json`,
		"missing confidence":  `{"description":"proposal sent"}`,
		"missing description": `{"confidence":0.5}`,
	}
	for name, detail := range cases {
		t.Run(name, func(t *testing.T) {
			view := domain.AssembledView{Facts: []domain.Fact{
				{EntityType: "proposal_sent_claim", EntityID: "deal:9", Detail: detail, Source: "capture:stage:9"},
			}}
			out, err := app.ProduceProposalSentFlags(view)
			if err != nil {
				t.Fatalf("ProduceProposalSentFlags: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 flags for a malformed claim, got %d", name, len(out))
			}
		})
	}
}
