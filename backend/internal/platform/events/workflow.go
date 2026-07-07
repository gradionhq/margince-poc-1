package events

import (
	"context"
	"encoding/json"
	"time"
)

// EventEnvelope is the canonical event carrier (formerly internal/shared/ports/workflow).
// Dependency-free — no River, Redis, or any Tier-1 import.
type EventEnvelope struct {
	ID          string          `json:"id"`
	Topic       string          `json:"topic"`
	WorkspaceID string          `json:"workspace_id"`
	EntityID    string          `json:"entity_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// Handler reacts to a domain event (formerly internal/shared/ports/workflow).
type Handler interface {
	Match(event string) bool
	Plan(ctx context.Context, event string, payload any) error
}

var registry []Handler

// Register adds h to the global workflow registry.
func Register(h Handler) { registry = append(registry, h) }

// All returns every registered Handler.
func All() []Handler { return registry }
