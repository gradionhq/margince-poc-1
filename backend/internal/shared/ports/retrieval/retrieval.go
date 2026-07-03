// Package retrieval is the Tier-0 seam for search and context assembly.
// crm-ai reaches crm-search through this seam ONLY (ADR-0007).
// Implemented by crm-search (hybrid tsvector + pgvector + context graph);
// crm-ai never imports crm-search internals.
package retrieval

import "context"

// HybridQuery is the option struct for a fused hybrid search.
type HybridQuery struct {
	Text      string    // the user query text (also the FTS query)
	Limit     int       // max results
	Rerank    bool      // enable the optional cross-encoder reranker (default false)
	Embedding []float32 // optional precomputed query embedding; if nil the engine generates one
}

// Retriever is the search + context-assembly seam.
type Retriever interface {
	Search(ctx context.Context, query string, limit int) ([]Result, error)
	HybridSearch(ctx context.Context, q HybridQuery) ([]Result, error)
	AssembleContext(ctx context.Context, entityID string) (Context, error)
}

// Result is a single search hit.
type Result struct {
	EntityType string
	EntityID   string
	Score      float64
	Snippet    string
	// citations
	SourceType string // e.g. "activity" — the record the snippet/citation points at
	SourceID   string // the clickable source-record id
	// Note: the trust tier of a hit is NOT carried here. Tier flows through the
	// provider seam (datasource.TierOf → WrapWithTier) at the tool layer, which is the
	// only live writer; a field here had no live writer or reader (it leaked the
	// seam's responsibility into the result struct). See datasource.TierOf.
}

// Context is an assembled context bundle for an entity.
type Context struct {
	EntityID   string
	EntityType string
	Summary    string
	Raw        map[string]any
}
