package transport

import (
	"context"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/ports/retrieval"
)

func TestNoOpRetriever_AssembleContext_EmptyRaw(t *testing.T) {
	r := NewNoOpRetriever()
	ctx, err := r.AssembleContext(context.Background(), "deal-1")
	if err != nil {
		t.Fatalf("AssembleContext: %v", err)
	}
	if ctx.Raw != nil {
		t.Fatalf("expected a nil Raw bag from the no-op retriever, got %+v", ctx.Raw)
	}
	if got := decodeOfferLineSignals(ctx); len(got) != 0 {
		t.Fatalf("expected zero decoded signals for an empty context, got %+v", got)
	}
}

func TestDecodeOfferLineSignals_PopulatedRaw(t *testing.T) {
	signals := []domain.OfferLineSignal{{Description: "Consulting", Quantity: 2, Snippet: "s", SourceID: "activity-1"}}
	ctx := retrieval.Context{Raw: map[string]any{offerLineSignalsKey: signals}}
	got := decodeOfferLineSignals(ctx)
	if len(got) != 1 || got[0].Description != "Consulting" {
		t.Fatalf("expected the one signal decoded back out, got %+v", got)
	}
}

func TestDecodeOfferLineSignals_WrongShape_EmptyNotPanic(t *testing.T) {
	ctx := retrieval.Context{Raw: map[string]any{offerLineSignalsKey: "not-a-slice"}}
	if got := decodeOfferLineSignals(ctx); len(got) != 0 {
		t.Fatalf("expected zero signals for a malformed Raw value, got %+v", got)
	}
}
