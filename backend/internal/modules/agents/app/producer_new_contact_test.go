package app_test

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func newContactFact(detail string) domain.Fact {
	return domain.Fact{EntityType: app.EntityTypeNewContactSignal, EntityID: "fact-1", Detail: detail, Source: "capture:email:1"}
}

func TestProduceNewContacts_CompleteSignalProducesAProposal(t *testing.T) {
	detail := mustJSON(map[string]any{
		"name": "Jane Roe", "email": "jane@example.com", "relates_to": "deal:deal-1",
		"evidence": "email signature block", "confidence": 0.85,
	})
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{newContactFact(detail)}}
	out, err := app.ProduceNewContacts(view)
	if err != nil {
		t.Fatalf("ProduceNewContacts: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d proposals, want 1", len(out))
	}
	p := out[0]
	if p.ActionType != "create_contact" || p.TargetEntity != "deal:deal-1" {
		t.Fatalf("ActionType/TargetEntity = %q/%q", p.ActionType, p.TargetEntity)
	}
	if p.Evidence == "" || p.Confidence == nil || p.Source == "" {
		t.Fatalf("no-guess fields missing: %+v", p)
	}
}

func TestProduceNewContacts_PhoneOnlyAlsoSucceeds(t *testing.T) {
	detail := mustJSON(map[string]any{
		"name": "Jane Roe", "phone": "+1-555-0100", "relates_to": "deal:deal-1",
		"evidence": "voicemail transcript", "confidence": 0.7,
	})
	view := domain.AssembledView{Facts: []domain.Fact{newContactFact(detail)}}
	out, err := app.ProduceNewContacts(view)
	if err != nil {
		t.Fatalf("ProduceNewContacts: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("phone-only signal: got %d proposals, want 1", len(out))
	}
}

func TestProduceNewContacts_MissingNameSkips(t *testing.T) {
	detail := mustJSON(map[string]any{
		"email": "jane@example.com", "relates_to": "deal:deal-1", "evidence": "e", "confidence": 0.5,
	})
	view := domain.AssembledView{Facts: []domain.Fact{newContactFact(detail)}}
	out, err := app.ProduceNewContacts(view)
	if err != nil {
		t.Fatalf("ProduceNewContacts: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("missing name: expected 0 proposals, got %d", len(out))
	}
}

func TestProduceNewContacts_MissingBothEmailAndPhoneSkips(t *testing.T) {
	detail := mustJSON(map[string]any{
		"name": "Jane Roe", "relates_to": "deal:deal-1", "evidence": "e", "confidence": 0.5,
	})
	view := domain.AssembledView{Facts: []domain.Fact{newContactFact(detail)}}
	out, err := app.ProduceNewContacts(view)
	if err != nil {
		t.Fatalf("ProduceNewContacts: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("missing email+phone: expected 0 proposals, got %d", len(out))
	}
}

func TestProduceNewContacts_IgnoresUnrelatedEntityType(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{{EntityType: "field_change_signal", Detail: "{}"}}}
	out, err := app.ProduceNewContacts(view)
	if err != nil {
		t.Fatalf("ProduceNewContacts: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected unrelated EntityType ignored, got %d proposals", len(out))
	}
}
