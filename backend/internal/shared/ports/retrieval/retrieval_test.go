package retrieval

import (
	"context"
	"testing"
)

type r struct{}

func (r) Search(_ context.Context, _ string, _ int) ([]Result, error) { return nil, nil }
func (r) AssembleContext(_ context.Context, _ string) (Context, error) {
	return Context{}, nil
}
func (r) HybridSearch(_ context.Context, _ HybridQuery) ([]Result, error) { return nil, nil }

func TestRetriever(t *testing.T) {
	var _ Retriever = r{}
}

func TestResultCitationFields(t *testing.T) {
	res := Result{EntityType: "person", EntityID: "p1", SourceType: "activity", SourceID: "a1"}
	if res.SourceType != "activity" || res.SourceID != "a1" {
		t.Fatalf("citation fields not set: %+v", res)
	}
}

func TestHybridQueryDefaults(t *testing.T) {
	var q HybridQuery
	if q.Rerank {
		t.Fatal("Rerank must default to false")
	}
}
