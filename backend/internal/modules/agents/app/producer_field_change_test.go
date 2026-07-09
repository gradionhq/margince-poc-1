package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func fieldChangeFact(detail string) domain.Fact {
	return domain.Fact{EntityType: app.EntityTypeFieldChangeSignal, EntityID: "fact-1", Detail: detail, Source: "capture:call:1"}
}

func mustJSON(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestProduceFieldChanges_AllowedFieldsEachProduceAProposal(t *testing.T) {
	cases := []struct {
		field, value string
	}{
		{"stage", `"negotiation"`},
		{"next_step", `"schedule demo"`},
		{"amount", `50000`},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			detail := mustJSON(map[string]any{
				"deal_id": "deal-1", "field": tc.field, "value": json.RawMessage(tc.value),
				"evidence": "call transcript mentions it", "confidence": 0.8,
			})
			view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{fieldChangeFact(detail)}}
			out, err := app.ProduceFieldChanges(view)
			if err != nil {
				t.Fatalf("ProduceFieldChanges: %v", err)
			}
			if len(out) != 1 {
				t.Fatalf("got %d proposals, want 1", len(out))
			}
			p := out[0]
			if p.ActionType != "field_change" || p.TargetEntity != "deal:deal-1" {
				t.Fatalf("ActionType/TargetEntity = %q/%q", p.ActionType, p.TargetEntity)
			}
			if p.Evidence == "" || p.Confidence == nil || p.Source == "" {
				t.Fatalf("no-guess fields missing: %+v", p)
			}
		})
	}
}

func TestProduceFieldChanges_CloseDateFieldProducesNothing(t *testing.T) {
	detail := mustJSON(map[string]any{
		"deal_id": "deal-1", "field": "close_date", "value": json.RawMessage(`"2026-08-01"`),
		"evidence": "e", "confidence": 0.8,
	})
	view := domain.AssembledView{Facts: []domain.Fact{fieldChangeFact(detail)}}
	out, err := app.ProduceFieldChanges(view)
	if err != nil {
		t.Fatalf("ProduceFieldChanges: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("close_date signal must never produce a field_change proposal (ONA-T02 owns it), got %d", len(out))
	}
}

func TestProduceFieldChanges_MalformedOrIncompleteSignalsSkip(t *testing.T) {
	cases := map[string]string{
		"unparseable detail": `not json`,
		"missing evidence":   mustJSON(map[string]any{"deal_id": "deal-1", "field": "stage", "confidence": 0.5}),
		"missing confidence":  mustJSON(map[string]any{"deal_id": "deal-1", "field": "stage", "evidence": "e"}),
		"unknown field":      mustJSON(map[string]any{"deal_id": "deal-1", "field": "owner", "evidence": "e", "confidence": 0.5}),
		"missing deal id":    mustJSON(map[string]any{"field": "stage", "evidence": "e", "confidence": 0.5}),
	}
	for name, detail := range cases {
		t.Run(name, func(t *testing.T) {
			view := domain.AssembledView{Facts: []domain.Fact{fieldChangeFact(detail)}}
			out, err := app.ProduceFieldChanges(view)
			if err != nil {
				t.Fatalf("ProduceFieldChanges: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 proposals, got %d", name, len(out))
			}
		})
	}
}

func TestProduceFieldChanges_IgnoresUnrelatedEntityType(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{{EntityType: "new_contact_signal", Detail: "{}"}}}
	out, err := app.ProduceFieldChanges(view)
	if err != nil {
		t.Fatalf("ProduceFieldChanges: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unrelated EntityType ignored, got %d proposals", len(out))
	}
}
