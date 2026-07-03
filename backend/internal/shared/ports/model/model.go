// Package model is the Tier-0 provider-agnostic LLM seam (ADR-0012/0020:
// both local-default and cloud-default bindings, customer-supplied inference).
// This package imports NO vendor SDK; vendor names appear only in adapter packages.
package model

import (
	"context"
	"encoding/json"
)

// Client is the inference seam crm-ai depends on.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
	Embed(ctx context.Context, req EmbedRequest) (Embeddings, error)
	Caps() Capabilities
}

// Request carries all inputs for a completion or streaming call.
type Request struct {
	Task           string
	System         string
	Prompt         string
	Schema         json.RawMessage
	MaxTokens      int
	SecretStripper SecretStripper
}

// Response is the result of a Complete call.
type Response struct {
	Text      string
	TokensIn  int
	TokensOut int
	Model     string
	Cached    bool
}

// Chunk is a single token delta from a Stream call.
type Chunk struct {
	Text string
	Err  error
}

// EmbedRequest carries inputs for an embedding call. SecretStripper, when set,
// redacts credentials from each input before it egresses — the same choke point
// Request uses, so the embedding path cannot leak unredacted text.
type EmbedRequest struct {
	Inputs         []string
	SecretStripper SecretStripper
}

// Embeddings is the result of an Embed call.
type Embeddings struct {
	Vectors [][]float32
	Dims    int
}

// Capabilities describes what a Client implementation supports.
type Capabilities struct {
	Streaming bool
	EmbedDims int
	LocalOnly bool
}

// SecretStripper redacts credentials from outbound payloads before any model call.
type SecretStripper interface{ Strip(payload string) string }
