package app_test

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func TestReconciliationProduce_ConcatenatesAllThreeProducersInOrder(t *testing.T) {
	fc := mustJSON(map[string]any{"deal_id": "d1", "field": "stage", "evidence": "e", "confidence": 0.5})
	nc := mustJSON(map[string]any{"name": "Jane", "email": "jane@x.com", "relates_to": "deal:d1", "evidence": "e", "confidence": 0.5})
	fu := mustJSON(map[string]any{"target": "deal:d1", "body": "hi", "evidence": "e", "confidence": 0.5})

	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		{EntityType: app.EntityTypeFieldChangeSignal, Detail: fc, Source: "s"},
		{EntityType: app.EntityTypeNewContactSignal, Detail: nc, Source: "s"},
		{EntityType: app.EntityTypeFollowUpSignal, Detail: fu, Source: "s"},
	}}

	out, err := app.ReconciliationProduce(view)
	if err != nil {
		t.Fatalf("ReconciliationProduce: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("got %d proposals, want 3", len(out))
	}
	wantOrder := []string{"field_change", "create_contact", "draft_followup"}
	for i, want := range wantOrder {
		if out[i].ActionType != want {
			t.Fatalf("out[%d].ActionType = %q, want %q (order: field changes, new contacts, follow-ups)", i, out[i].ActionType, want)
		}
	}
}

func TestReconciliationProduce_NeverReturnsAnError(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		{EntityType: app.EntityTypeFieldChangeSignal, Detail: "not json"},
		{EntityType: "unrelated"},
	}}
	out, err := app.ReconciliationProduce(view)
	if err != nil {
		t.Fatalf("ReconciliationProduce must never return an error, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 proposals from malformed/unrelated facts, got %d", len(out))
	}
}
