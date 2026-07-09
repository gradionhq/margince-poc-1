package transport

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/shared/ports/retrieval"
)

// NoOpRetriever is the offers module's minimal, honest retrieval.Retriever
// implementation for when no search backend is wired.
type NoOpRetriever struct{}

// NewNoOpRetriever returns a NoOpRetriever.
func NewNoOpRetriever() *NoOpRetriever { return &NoOpRetriever{} }

// Search always returns no results — no search backend is wired (see the
// type doc comment).
func (NoOpRetriever) Search(_ context.Context, _ string, _ int) ([]retrieval.Result, error) {
	return nil, nil
}

// HybridSearch always returns no results — no search backend is wired (see
// the type doc comment).
func (NoOpRetriever) HybridSearch(_ context.Context, _ retrieval.HybridQuery) ([]retrieval.Result, error) {
	return nil, nil
}

// AssembleContext always returns an empty deal-typed context — no search
// backend is wired (see the type doc comment).
func (NoOpRetriever) AssembleContext(_ context.Context, entityID string) (retrieval.Context, error) {
	return retrieval.Context{EntityID: entityID, EntityType: "deal"}, nil
}

var _ retrieval.Retriever = (*NoOpRetriever)(nil)
