package workflow

import (
	"encoding/json"
	"time"
)

// EventEnvelope is the Tier-0 canonical event carrier. It is dependency-free —
// no River, Redis, or any Tier-1 import is allowed here.
type EventEnvelope struct {
	ID          string          `json:"id"`
	Topic       string          `json:"topic"`
	WorkspaceID string          `json:"workspace_id"`
	EntityID    string          `json:"entity_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}
