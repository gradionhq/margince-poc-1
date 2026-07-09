package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func TestProduceStageUnsupportedFlags_SupportedStageYieldsNeitherFlagNorCorrection(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		{EntityType: "stage_claim", EntityID: "deal:1", Detail: `{"confidence":0.9,"stage":"negotiation"}`, Source: "capture:stage:1"},
		{EntityType: "stage_signal", EntityID: "deal:1", Detail: `{"confidence":0.88,"supports_stage":"negotiation"}`, Source: "capture:signal:1"},
	}}
	out, err := app.ProduceStageUnsupportedFlags(view)
	if err != nil {
		t.Fatalf("ProduceStageUnsupportedFlags: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 proposals for a supported stage claim, got %d: %+v", len(out), out)
	}
}

func TestProduceStageUnsupportedFlags_UnsupportedWithSignalYieldsFlagAndCorrection(t *testing.T) {
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		{EntityType: "stage_claim", EntityID: "deal:2", Detail: `{"confidence":0.7,"stage":"proposal_sent"}`, Source: "capture:stage:2"},
		{EntityType: "stage_signal", EntityID: "deal:2", Detail: `{"confidence":0.6,"supports_stage":"discovery"}`, Source: "capture:signal:2a"},
		{EntityType: "stage_signal", EntityID: "deal:2", Detail: `{"confidence":0.82,"supports_stage":"negotiation"}`, Source: "capture:signal:2b"},
	}}
	out, err := app.ProduceStageUnsupportedFlags(view)
	if err != nil {
		t.Fatalf("ProduceStageUnsupportedFlags: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 1 flag + 1 correction, got %d: %+v", len(out), out)
	}
	flag, correction := out[0], out[1]
	if flag.ActionType != "integrity_flag" {
		t.Errorf("out[0].ActionType = %q, want integrity_flag", flag.ActionType)
	}
	if flag.Confidence == nil || *flag.Confidence != 0.7 {
		t.Errorf("flag Confidence = %v, want 0.7 (the claim's own)", flag.Confidence)
	}
	if correction.ActionType != "stage_correction" {
		t.Errorf("out[1].ActionType = %q, want stage_correction", correction.ActionType)
	}
	var effect map[string]string
	if err := json.Unmarshal(correction.Effect, &effect); err != nil {
		t.Fatalf("Effect unmarshal: %v", err)
	}
	if effect["field"] != "stage" || effect["value"] != "negotiation" {
		t.Errorf("correction Effect = %+v, want field=stage value=negotiation (highest-confidence signal)", effect)
	}
	if correction.Confidence == nil || *correction.Confidence != 0.82 {
		t.Errorf("correction Confidence = %v, want 0.82 (the signal's own, never the claim's)", correction.Confidence)
	}
	if correction.Source != "capture:signal:2b" {
		t.Errorf("correction Source = %q, want capture:signal:2b", correction.Source)
	}
	if correction.TargetEntity != "deal:2" {
		t.Errorf("correction TargetEntity = %q, want deal:2", correction.TargetEntity)
	}
}

func TestProduceStageUnsupportedFlags_UnsupportedWithNoSignalYieldsFlagOnly(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		{EntityType: "stage_claim", EntityID: "deal:3", Detail: `{"confidence":0.65,"stage":"proposal_sent"}`, Source: "capture:stage:3"},
	}}
	out, err := app.ProduceStageUnsupportedFlags(view)
	if err != nil {
		t.Fatalf("ProduceStageUnsupportedFlags: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 flag, 0 corrections (no signal to correct from), got %d: %+v", len(out), out)
	}
	if out[0].ActionType != "integrity_flag" {
		t.Errorf("ActionType = %q, want integrity_flag", out[0].ActionType)
	}
}

func TestProduceStageUnsupportedFlags_MalformedClaimYieldsNeither(t *testing.T) {
	cases := map[string]string{
		"not json":           `not-json`,
		"missing confidence": `{"stage":"negotiation"}`,
		"missing stage":      `{"confidence":0.5}`,
	}
	for name, detail := range cases {
		t.Run(name, func(t *testing.T) {
			view := domain.AssembledView{Facts: []domain.Fact{
				{EntityType: "stage_claim", EntityID: "deal:9", Detail: detail, Source: "capture:stage:9"},
			}}
			out, err := app.ProduceStageUnsupportedFlags(view)
			if err != nil {
				t.Fatalf("ProduceStageUnsupportedFlags: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 proposals for a malformed claim, got %d", name, len(out))
			}
		})
	}
}
