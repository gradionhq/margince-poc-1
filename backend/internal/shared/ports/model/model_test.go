package model

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeClient struct{}

func (fakeClient) Complete(context.Context, Request) (Response, error) {
	return Response{Text: "ok", Model: "fake"}, nil
}

func (fakeClient) Stream(context.Context, Request) (<-chan Chunk, error) {
	ch := make(chan Chunk, 1)
	ch <- Chunk{Text: "ok"}
	close(ch)
	return ch, nil
}

func (fakeClient) Embed(context.Context, EmbedRequest) (Embeddings, error) {
	return Embeddings{Vectors: [][]float32{{0.1, 0.2}}, Dims: 2}, nil
}
func (fakeClient) Caps() Capabilities { return Capabilities{Streaming: true, EmbedDims: 2} }

func TestClientImplemented(t *testing.T) {
	var c Client = fakeClient{}
	r, err := c.Complete(context.Background(), Request{Task: "t", Prompt: "p"})
	if err != nil || r.Text != "ok" {
		t.Fatalf("Complete: %v %q", err, r.Text)
	}
	if c.Caps().EmbedDims != 2 {
		t.Fatal("Caps round-trip failed")
	}
	_ = json.RawMessage(nil)
}
