package extraction

import (
	"context"
	"testing"
)

func TestNoOpExtractor(t *testing.T) {
	var ex Extractor = NoOpExtractor{}

	fields, err := ex.Extract(context.Background(), "any-id")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(fields) != 0 {
		t.Fatalf("Extract returned %d fields, want 0", len(fields))
	}
}

func TestFixtureExtractor(t *testing.T) {
	seeded := []ExtractedField{{
		Field:       "name",
		Value:       "Acme Corp",
		SourceQuote: "Acme Corp",
		Confidence:  "high",
	}}
	ex := FixtureExtractor{Fields: map[string][]ExtractedField{
		"att-1": seeded,
	}}

	fields, err := ex.Extract(context.Background(), "att-1")
	if err != nil {
		t.Fatalf("Extract seeded: %v", err)
	}
	if len(fields) != len(seeded) {
		t.Fatalf("Extract seeded returned %d fields, want %d", len(fields), len(seeded))
	}
	if fields[0] != seeded[0] {
		t.Fatalf("Extract seeded returned %+v, want %+v", fields[0], seeded[0])
	}

	unknown, err := ex.Extract(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("Extract unknown: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("Extract unknown returned %d fields, want 0", len(unknown))
	}
}
