// Package domain contains the pure formatting and masking logic for audit history entries.
package domain

import (
	"fmt"
	"time"
)

// Local constants for actor types and action values used in audit formatting.
const (
	actorTypeAgent     = "agent"
	actorTypeConnector = "connector"
	actionApproved     = "approved"
	actionPromoted     = "promoted"
)

// AuditHistoryEntry is one rendered history line for a record mutation.
// before/after are field-masked to the viewer's readable fields.
type AuditHistoryEntry struct {
	ID                string         `json:"id"`
	ActorType         string         `json:"actor_type"`
	ActorID           string         `json:"actor_id"`
	OnBehalfOf        *string        `json:"on_behalf_of,omitempty"`
	OnBehalfOfName    *string        `json:"on_behalf_of_name,omitempty"`
	Action            string         `json:"action"`
	OccurredAt        time.Time      `json:"occurred_at"`
	AuthorizationRule *string        `json:"authorization_rule,omitempty"`
	Before            map[string]any `json:"before,omitempty"`
	After             map[string]any `json:"after,omitempty"`
	Summary           string         `json:"summary"`
}

// EntityFieldMask is the set of field names to hide in before/after for an entity type.
// An absent field in the mask is visible; a present one is removed from the output.
type EntityFieldMask map[string]struct{}

// DefaultFieldMasks is the minimal per-entity-type mask required by AC2.
// Extend here when a future story adds field-level RBAC columns.
// An absent entry (or nil mask) means no fields are masked.
var DefaultFieldMasks = map[string]EntityFieldMask{
	// No fields are masked by default in V1; the seam is here for AC2 tests
	// to inject a non-empty mask. Future stories populate this map.
}

// ApplyFieldMask returns a copy of data with every key in mask removed.
// Returns nil when data is nil.
func ApplyFieldMask(data map[string]any, mask EntityFieldMask) map[string]any {
	if data == nil {
		return nil
	}
	if len(mask) == 0 {
		out := make(map[string]any, len(data))
		for k, v := range data {
			out[k] = v
		}
		return out
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		if _, hidden := mask[k]; !hidden {
			out[k] = v
		}
	}
	return out
}

// ComposeSummary builds a plain-language description of one audit event.
// actorDisplayName is the human-readable name of the actor (e.g. display_name from app_user,
// or actor_id when no name is available). onBehalfOfName is set only for agent actions.
func ComposeSummary(actorType, actorDisplayName string, onBehalfOfName *string, action string) string {
	actionWord := actionToVerb(action)
	switch actorType {
	case actorTypeAgent:
		if onBehalfOfName != nil && *onBehalfOfName != "" {
			return fmt.Sprintf("Agent acting for %s %s the record", *onBehalfOfName, actionWord)
		}
		return fmt.Sprintf("Agent %s the record", actionWord)
	case "human":
		return fmt.Sprintf("%s %s the record", actorDisplayName, actionWord)
	case "system":
		return fmt.Sprintf("System %s the record", actionWord)
	case actorTypeConnector:
		return fmt.Sprintf("Connector %s the record", actionWord)
	default:
		return fmt.Sprintf("%s %s the record", actorType, actionWord)
	}
}

// actionVerbs maps an audit action to its past-tense verb phrase. Unknown actions
// fall back to the raw action string (see actionToVerb).
var actionVerbs = map[string]string{
	"create":           "created",
	"update":           "updated",
	"archive":          "archived",
	"merge":            "merged",
	"promote":          actionPromoted,
	"disqualify":       "disqualified",
	"restore":          "restored",
	"export":           "exported",
	"erase":            "erased",
	"anonymize":        "anonymized",
	"login":            "logged in",
	"assign":           "assigned",
	"advance_stage":    "advanced the stage of",
	"send_email":       "sent an email for",
	"consent_grant":    "granted consent for",
	"consent_withdraw": "withdrew consent for",
	"approve":          actionApproved,
	"reject":           "rejected",
	"record_share":     "shared",
	"record_unshare":   "unshared",
}

// actionToVerb converts an audit action to a past-tense verb phrase.
func actionToVerb(action string) string {
	if verb, ok := actionVerbs[action]; ok {
		return verb
	}
	return action
}
