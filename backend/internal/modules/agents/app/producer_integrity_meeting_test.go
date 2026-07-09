package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func TestProduceMeetingRecapFlags_MatchingRecapYieldsZeroFlags(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		{EntityType: "meeting_claim", EntityID: "deal:1", Detail: `{"confidence":0.8,"description":"discovery meeting logged"}`, Source: "capture:meeting:1"},
		{EntityType: "meeting_recap_trace", EntityID: "deal:1", Source: "capture:recap:1"},
	}}
	out, err := app.ProduceMeetingRecapFlags(view)
	if err != nil {
		t.Fatalf("ProduceMeetingRecapFlags: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 flags for a recapped meeting, got %d: %+v", len(out), out)
	}
}

func TestProduceMeetingRecapFlags_NoRecapYieldsOneFlag(t *testing.T) {
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		{EntityType: "meeting_claim", EntityID: "deal:2", Detail: `{"confidence":0.65,"description":"discovery meeting logged"}`, Source: "capture:meeting:2"},
	}}
	out, err := app.ProduceMeetingRecapFlags(view)
	if err != nil {
		t.Fatalf("ProduceMeetingRecapFlags: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 flag for a recap-less meeting, got %d", len(out))
	}
	p := out[0]
	if p.ActionType != "integrity_flag" || p.TargetEntity != "deal:2" || p.Evidence == "" {
		t.Fatalf("unexpected proposal shape: %+v", p)
	}
	if p.Confidence == nil || *p.Confidence != 0.65 {
		t.Errorf("Confidence = %v, want 0.65", p.Confidence)
	}
	var effect map[string]string
	if err := json.Unmarshal(p.Effect, &effect); err != nil {
		t.Fatalf("Effect unmarshal: %v", err)
	}
	if effect["check"] != "meeting_without_recap" || effect["claim"] != "discovery meeting logged" {
		t.Errorf("Effect = %+v", effect)
	}
}

func TestProduceMeetingRecapFlags_MalformedClaimYieldsZeroFlags(t *testing.T) {
	cases := map[string]string{
		"not json":            `not-json`,
		"missing confidence":  `{"description":"meeting logged"}`,
		"missing description": `{"confidence":0.5}`,
	}
	for name, detail := range cases {
		t.Run(name, func(t *testing.T) {
			view := domain.AssembledView{Facts: []domain.Fact{
				{EntityType: "meeting_claim", EntityID: "deal:9", Detail: detail, Source: "capture:meeting:9"},
			}}
			out, err := app.ProduceMeetingRecapFlags(view)
			if err != nil {
				t.Fatalf("ProduceMeetingRecapFlags: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 flags for a malformed claim, got %d", name, len(out))
			}
		})
	}
}
