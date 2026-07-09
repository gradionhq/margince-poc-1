package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func TestProduceUntracedCallFlags_MatchingTraceYieldsZeroFlags(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		{EntityType: "call_claim", EntityID: "deal:1", Detail: `{"confidence":0.9,"description":"call logged"}`, Source: "capture:call:1"},
		{EntityType: "call_trace", EntityID: "deal:1", Source: "capture:calendar:1"},
	}}
	out, err := app.ProduceUntracedCallFlags(view)
	if err != nil {
		t.Fatalf("ProduceUntracedCallFlags: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 flags for a corroborated call, got %d: %+v", len(out), out)
	}
}

func TestProduceUntracedCallFlags_NoTraceYieldsOneFlag(t *testing.T) {
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		{EntityType: "call_claim", EntityID: "deal:2", Detail: `{"confidence":0.75,"description":"call re: pricing"}`, Source: "capture:call:2"},
	}}
	out, err := app.ProduceUntracedCallFlags(view)
	if err != nil {
		t.Fatalf("ProduceUntracedCallFlags: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 flag for an untraced call, got %d", len(out))
	}
	p := out[0]
	if p.ActionType != "integrity_flag" {
		t.Errorf("ActionType = %q, want integrity_flag", p.ActionType)
	}
	if p.TargetEntity != "deal:2" {
		t.Errorf("TargetEntity = %q, want deal:2", p.TargetEntity)
	}
	if p.Evidence == "" {
		t.Error("Evidence must be non-empty")
	}
	if p.Confidence == nil || *p.Confidence != 0.75 {
		t.Errorf("Confidence = %v, want 0.75", p.Confidence)
	}
	if p.Source != "capture:call:2" {
		t.Errorf("Source = %q, want capture:call:2", p.Source)
	}
	var effect map[string]string
	if err := json.Unmarshal(p.Effect, &effect); err != nil {
		t.Fatalf("Effect unmarshal: %v", err)
	}
	if effect["check"] != "untraced_call" || effect["claim"] != "call re: pricing" {
		t.Errorf("Effect = %+v, want check=untraced_call claim=%q", effect, "call re: pricing")
	}
}

func TestProduceUntracedCallFlags_MalformedClaimYieldsZeroFlags(t *testing.T) {
	cases := map[string]string{
		"not json":            `not-json`,
		"missing confidence":  `{"description":"call logged"}`,
		"missing description": `{"confidence":0.5}`,
	}
	for name, detail := range cases {
		t.Run(name, func(t *testing.T) {
			view := domain.AssembledView{Facts: []domain.Fact{
				{EntityType: "call_claim", EntityID: "deal:9", Detail: detail, Source: "capture:call:9"},
			}}
			out, err := app.ProduceUntracedCallFlags(view)
			if err != nil {
				t.Fatalf("ProduceUntracedCallFlags: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 flags for a malformed claim, got %d", name, len(out))
			}
		})
	}
}
