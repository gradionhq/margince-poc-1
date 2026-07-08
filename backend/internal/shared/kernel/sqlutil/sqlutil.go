// Package sqlutil is the Tier-0 kernel of generic, domain-free helpers shared by
// the per-entity modules' PostgreSQL storage adapters. These are byte-for-byte
// identical utilities (provenance guard, JSON (un)marshalling of event/audit
// payloads, bounded-update field readers, opaque pagination cursors) that were
// copy-pasted into every catalog module during the directory-split (WS-E-b) and
// are consolidated here so no module carries its own copy.
//
// Nothing domain-specific belongs here: entity-type strings, field-name
// constants and business logic stay in each module's adapters package.
package sqlutil

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// RequireProvenance rejects an empty source or captured_by with a typed sentinel
// (data-model §1.6 provenance). HTTP handlers already reject empties at the edge,
// but non-HTTP callers (import/Datasource/direct store use) must not be able to
// insert source="" or captured_by="" — provenance is a load-bearing invariant.
func RequireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

// MarshalJSON encodes v to a JSON byte slice, returning "{}" for a nil value so
// a nullable jsonb column is never written as SQL NULL.
func MarshalJSON(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(v)
	return b
}

// UnmarshalJSON decodes raw into dst, tolerating a nil/empty input (leaves dst
// untouched) and malformed JSON (best-effort, error ignored).
func UnmarshalJSON(raw []byte, dst *map[string]any) {
	if raw == nil {
		return
	}
	_ = json.Unmarshal(raw, dst)
}

// NullStr reads a *string from a bounded-update map; nil when the key is absent
// or the value is not a string.
func NullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

// NullStrParam binds s as a SQL value, or NULL when s is empty.
func NullStrParam(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// EncodeOffsetCursor packs an in-memory offset into an opaque, URL-safe token.
func EncodeOffsetCursor(n int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(n)))
}

// DecodeOffsetCursor unpacks a token from EncodeOffsetCursor; returns 0 (first
// page) for an empty or malformed token.
func DecodeOffsetCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// EncodeKeysetCursor packs (sortVal, id) into one opaque, URL-safe token.
func EncodeKeysetCursor(sortVal, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortVal + "\x00" + id))
}

// DecodeKeysetCursor unpacks a token from EncodeKeysetCursor. ok=false for an
// empty or malformed token, in which case the caller treats it as "first page".
func DecodeKeysetCursor(cursor string) (sortVal, id string, ok bool) {
	if cursor == "" {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
