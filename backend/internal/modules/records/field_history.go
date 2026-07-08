package records

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	audithistorydomain "github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
)

// FieldHistoryEntry is one per-field change projected read-only from a single audit_log row's
// before/after diff (RD-WIRE-5 / RD-AC-5). id is the source audit_log row's id.
// old_value/new_value are server-rendered strings. No omitempty on nullable pointer/map fields —
// the contract requires explicit null serialization, not key omission.
type FieldHistoryEntry struct {
	ID         string         `json:"id"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Field      string         `json:"field"`
	OldValue   *string        `json:"old_value"`
	NewValue   *string        `json:"new_value"`
	ChangedAt  time.Time      `json:"changed_at"`
	ActorType  string         `json:"actor_type"`
	ActorID    string         `json:"actor_id"`
	PassportID *string        `json:"passport_id"`
	Evidence   map[string]any `json:"evidence"`
}

// auditLogRow carries the fields from one audit_log row needed by the diff transform.
type auditLogRow struct {
	id         string
	entityType string
	entityID   string
	actorType  string
	actorID    string
	passportID *string
	evidence   map[string]any
	occurredAt time.Time
	before     map[string]any
	after      map[string]any
}

// diffRowFields projects one audit_log row into per-field FieldHistoryEntry values (RD-AC-5).
// mask hides fields via total withholding (matching live-value masking). fieldFilter, if non-nil,
// narrows to only the named field — applied inside the loop before any row is counted toward the
// page. passport_id/evidence surface only when actor_type == "agent".
func diffRowFields(row auditLogRow, mask audithistorydomain.EntityFieldMask, fieldFilter *string) []FieldHistoryEntry {
	maskedBefore := audithistorydomain.ApplyFieldMask(row.before, mask)
	maskedAfter := audithistorydomain.ApplyFieldMask(row.after, mask)

	// union of all keys from both sides, sorted alphabetically for deterministic emission order
	keyset := make(map[string]struct{}, len(maskedBefore)+len(maskedAfter))
	for k := range maskedBefore {
		keyset[k] = struct{}{}
	}
	for k := range maskedAfter {
		keyset[k] = struct{}{}
	}
	keys := make([]string, 0, len(keyset))
	for k := range keyset {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var entries []FieldHistoryEntry
	for _, key := range keys {
		if fieldFilter != nil && key != *fieldFilter {
			continue
		}
		beforeVal, inBefore := maskedBefore[key]
		afterVal, inAfter := maskedAfter[key]

		switch {
		case inAfter && (!inBefore || !reflect.DeepEqual(beforeVal, afterVal)):
			// field added (create) or value changed
			entries = append(entries, makeFieldEntry(row, key, stringifyValue(beforeVal), stringifyValue(afterVal)))
		case inBefore && !inAfter:
			// field removed
			entries = append(entries, makeFieldEntry(row, key, stringifyValue(beforeVal), nil))
		// both present with equal values: emit nothing (RD-AC-5 no-fabricated-timeline)
		}
	}
	return entries
}

// makeFieldEntry builds one FieldHistoryEntry.
func makeFieldEntry(row auditLogRow, field string, oldValue, newValue *string) FieldHistoryEntry {
	var passportID *string
	var evidence map[string]any
	if row.actorType == "agent" {
		passportID = row.passportID
		evidence = row.evidence
	}
	return FieldHistoryEntry{
		ID:         row.id,
		EntityType: row.entityType,
		EntityID:   row.entityID,
		Field:      field,
		OldValue:   oldValue,
		NewValue:   newValue,
		ChangedAt:  row.occurredAt,
		ActorType:  row.actorType,
		ActorID:    row.actorID,
		PassportID: passportID,
		Evidence:   evidence,
	}
}

// stringifyValue renders v as a *string via fmt.Sprintf("%v").
// A nil value returns nil (never the literal strings "nil" or "<nil>").
func stringifyValue(v any) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprintf("%v", v)
	return &s
}

// encodeCursor encodes (occurredAt, id) into an opaque base64 cursor string.
// Format: base64(RFC3339Nano "|" id).
func encodeCursor(occurredAt time.Time, id string) string {
	raw := occurredAt.UTC().Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor decodes an opaque cursor string.
//
// Returns (time.Time{}, "", true) when s is blank — the first-page sentinel; callers detect
// this by checking t.IsZero(). Returns (time.Time{}, "", false) when s is non-empty but
// malformed (caller should respond 400 invalid_cursor). Returns (t, id, true) for a valid cursor.
func decodeCursor(s string) (occurredAt time.Time, id string, ok bool) {
	if s == "" {
		return time.Time{}, "", true
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", false
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return time.Time{}, "", false
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", false
	}
	return t, parts[1], true
}
