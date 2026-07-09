package domain_test

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
)

func TestFilterGroundedSignals_DropsUngroundedKeepsGrounded(t *testing.T) {
	signals := []domain.OfferLineSignal{
		{Description: "Consulting", Quantity: 2, Snippet: "the customer asked for 2 days of consulting", SourceID: "activity-1"},
		{Description: "Missing snippet", Quantity: 1, Snippet: "", SourceID: "activity-2"},
		{Description: "Missing source", Quantity: 1, Snippet: "some text", SourceID: ""},
	}
	got := domain.FilterGroundedSignals(signals)
	if len(got) != 1 || got[0].Description != "Consulting" {
		t.Fatalf("expected exactly the one grounded signal, got %+v", got)
	}
}

func TestFilterGroundedSignals_EmptyInput_EmptyOutput(t *testing.T) {
	got := domain.FilterGroundedSignals(nil)
	if len(got) != 0 {
		t.Fatalf("expected zero signals for a nil/empty input, got %+v", got)
	}
}
