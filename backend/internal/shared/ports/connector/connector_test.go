package connector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizedRecordValidate(t *testing.T) {
	valid := NormalizedRecord{
		Kind:       "activity",
		NaturalKey: NaturalKey{SourceSystem: "email", SourceID: "msg-1"},
		Source:     "email:msg-1",
		CapturedBy: "connector:email",
		Payload:    json.RawMessage(`{}`),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
	cases := map[string]NormalizedRecord{
		"missing kind":          {NaturalKey: NaturalKey{SourceSystem: "e", SourceID: "1"}, Source: "e:1", CapturedBy: "connector:e"},
		"missing source_system": {Kind: "activity", NaturalKey: NaturalKey{SourceID: "1"}, Source: "e:1", CapturedBy: "connector:e"},
		"missing source_id":     {Kind: "activity", NaturalKey: NaturalKey{SourceSystem: "e"}, Source: "e:1", CapturedBy: "connector:e"},
		"missing source":        {Kind: "activity", NaturalKey: NaturalKey{SourceSystem: "e", SourceID: "1"}, CapturedBy: "connector:e"},
		"missing captured_by":   {Kind: "activity", NaturalKey: NaturalKey{SourceSystem: "e", SourceID: "1"}, Source: "e:1"},
	}
	for name, rec := range cases {
		if err := rec.Validate(); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}

// stubConnector exercises the Connector interface shape & Normalize purity.
type stubConnector struct{}

func (stubConnector) Descriptor() Descriptor {
	return Descriptor{Name: "stub", Scopes: []string{"read"}, Tier: "green"}
}

func (stubConnector) Normalize(_ context.Context, raw []byte) ([]NormalizedRecord, error) {
	if len(raw) == 0 {
		return nil, ErrSkip
	}
	return []NormalizedRecord{{
		Kind: "activity", NaturalKey: NaturalKey{SourceSystem: "stub", SourceID: "1"},
		Source: "stub:1", CapturedBy: "connector:stub", Payload: raw,
	}}, nil
}

func TestConnectorNormalizePureSkip(t *testing.T) {
	var c Connector = stubConnector{}
	if _, err := c.Normalize(context.Background(), nil); !errors.Is(err, ErrSkip) {
		t.Fatalf("expected ErrSkip on empty input, got %v", err)
	}
	recs, err := c.Normalize(context.Background(), []byte(`{"x":1}`))
	if err != nil || len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d err=%v", len(recs), err)
	}
}
