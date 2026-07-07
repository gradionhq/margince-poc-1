package domain

import (
	"testing"
)

func strPtr(s string) *string { return &s }

func TestComposeSummary_Human(t *testing.T) {
	got := ComposeSummary("human", "Alice", nil, "update")
	want := "Alice updated the record"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComposeSummary_Agent_WithAuthority(t *testing.T) {
	got := ComposeSummary("agent", "Bot", strPtr("Devin"), "archive")
	want := "Agent acting for Devin archived the record"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComposeSummary_Agent_NoAuthority(t *testing.T) {
	got := ComposeSummary("agent", "Bot", nil, "create")
	want := "Agent created the record"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComposeSummary_System(t *testing.T) {
	got := ComposeSummary("system", "system", nil, "export")
	want := "System exported the record"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyFieldMask_RemovesMaskedField(t *testing.T) {
	data := map[string]any{"stage": "Discovery", "amount": 1000, "internal_note": "secret"}
	mask := EntityFieldMask{"internal_note": struct{}{}}
	got := ApplyFieldMask(data, mask)
	if _, exists := got["internal_note"]; exists {
		t.Error("masked field 'internal_note' must be absent from result")
	}
	if got["stage"] != "Discovery" {
		t.Errorf("unmasked field 'stage' must be preserved, got %v", got["stage"])
	}
}

func TestApplyFieldMask_NilData(t *testing.T) {
	got := ApplyFieldMask(nil, EntityFieldMask{"foo": struct{}{}})
	if got != nil {
		t.Errorf("nil input must return nil, got %v", got)
	}
}

func TestApplyFieldMask_EmptyMask(t *testing.T) {
	data := map[string]any{"stage": "Discovery"}
	got := ApplyFieldMask(data, nil)
	if got["stage"] != "Discovery" {
		t.Errorf("empty mask must preserve all fields")
	}
}
