package app_test

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func followUpFact(detail string) domain.Fact {
	return domain.Fact{EntityType: app.EntityTypeFollowUpSignal, EntityID: "fact-1", Detail: detail, Source: "capture:call:1"}
}

func TestProduceFollowUps_CompleteSignalProducesAProposal(t *testing.T) {
	detail := mustJSON(map[string]any{
		"target": "deal:deal-1", "recipient": "jane@example.com", "subject": "Following up",
		"body": "Great chatting today...", "evidence": "call recap", "confidence": 0.8,
	})
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{followUpFact(detail)}}
	out, err := app.ProduceFollowUps(view)
	if err != nil {
		t.Fatalf("ProduceFollowUps: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d proposals, want 1", len(out))
	}
	p := out[0]
	if p.ActionType != "draft_followup" || p.TargetEntity != "deal:deal-1" {
		t.Fatalf("ActionType/TargetEntity = %q/%q", p.ActionType, p.TargetEntity)
	}
	if p.Evidence == "" || p.Confidence == nil || p.Source == "" {
		t.Fatalf("no-guess fields missing: %+v", p)
	}
}

func TestProduceFollowUps_EmptyDraftBodySkips(t *testing.T) {
	detail := mustJSON(map[string]any{
		"target": "deal:deal-1", "recipient": "jane@example.com", "subject": "Following up",
		"body": "", "evidence": "call recap", "confidence": 0.8,
	})
	view := domain.AssembledView{Facts: []domain.Fact{followUpFact(detail)}}
	out, err := app.ProduceFollowUps(view)
	if err != nil {
		t.Fatalf("ProduceFollowUps: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("empty draft body must never be invented: expected 0 proposals, got %d", len(out))
	}
}

func TestProduceFollowUps_MissingEvidenceOrConfidenceSkips(t *testing.T) {
	cases := map[string]string{
		"missing evidence":   mustJSON(map[string]any{"target": "deal:deal-1", "body": "hi", "confidence": 0.5}),
		"missing confidence": mustJSON(map[string]any{"target": "deal:deal-1", "body": "hi", "evidence": "e"}),
		"missing target":     mustJSON(map[string]any{"body": "hi", "evidence": "e", "confidence": 0.5}),
	}
	for name, detail := range cases {
		t.Run(name, func(t *testing.T) {
			view := domain.AssembledView{Facts: []domain.Fact{followUpFact(detail)}}
			out, err := app.ProduceFollowUps(view)
			if err != nil {
				t.Fatalf("ProduceFollowUps: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 proposals, got %d", name, len(out))
			}
		})
	}
}

func TestProduceFollowUps_IgnoresUnrelatedEntityType(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{{EntityType: "new_contact_signal", Detail: "{}"}}}
	out, err := app.ProduceFollowUps(view)
	if err != nil {
		t.Fatalf("ProduceFollowUps: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unrelated EntityType ignored, got %d proposals", len(out))
	}
}
