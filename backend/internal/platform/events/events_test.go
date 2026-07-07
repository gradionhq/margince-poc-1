package events

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEventEnvelopeJSONRoundTrip(t *testing.T) {
	env := EventEnvelope{
		ID:          "01970000-0000-7000-8000-000000000001",
		Topic:       "person.created",
		WorkspaceID: "01970000-0000-7000-8000-000000000002",
		EntityID:    "01970000-0000-7000-8000-000000000003",
		OccurredAt:  time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
		Payload:     json.RawMessage(`{"name":"Alice"}`),
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got EventEnvelope
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != env.ID {
		t.Errorf("ID: got %q want %q", got.ID, env.ID)
	}
	if got.Topic != env.Topic {
		t.Errorf("Topic: got %q want %q", got.Topic, env.Topic)
	}
	if got.WorkspaceID != env.WorkspaceID {
		t.Errorf("WorkspaceID: got %q want %q", got.WorkspaceID, env.WorkspaceID)
	}
	if got.EntityID != env.EntityID {
		t.Errorf("EntityID: got %q want %q", got.EntityID, env.EntityID)
	}
	if !got.OccurredAt.Equal(env.OccurredAt) {
		t.Errorf("OccurredAt: got %v want %v", got.OccurredAt, env.OccurredAt)
	}
	if !strings.Contains(string(got.Payload), "Alice") {
		t.Errorf("Payload: got %s", got.Payload)
	}
}

func TestEventEnvelopeTopicFormat(t *testing.T) {
	env := EventEnvelope{Topic: "person.created"}
	parts := strings.SplitN(env.Topic, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Errorf("Topic should be <entity>.<verb>, got %q", env.Topic)
	}
}

type h struct{}

func (h) Match(string) bool                       { return true }
func (h) Plan(context.Context, string, any) error { return nil }

func TestHandler(t *testing.T) {
	var _ Handler = h{}
}
