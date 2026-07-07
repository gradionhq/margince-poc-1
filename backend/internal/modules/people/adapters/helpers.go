// Package adapters — supplemental shared helpers for the people module's
// SQL adapters. The core helpers (requireProvenance, marshalJSON,
// unmarshalJSON, nullStr, entityTypePerson) live in store_person.go alongside
// the PersonStore they primarily serve. The offset-cursor helpers
// (encodeOffsetCursor, decodeOffsetCursor) live in store_strength.go.
// This file holds the remaining keyset-cursor and nil-coercion utilities
// available for future adapter files.
package adapters

import (
	"encoding/base64"
	"strings"
	"time"
)

// encodeKeysetCursor packs (sortVal, id) into one opaque, URL-safe token.
// A page ordered by (sortCol, id) must seek on the FULL sort key — both
// components are encoded so the cursor round-trips the whole key.
func encodeKeysetCursor(sortVal, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortVal + "\x00" + id))
}

// decodeKeysetCursor unpacks a token from encodeKeysetCursor. ok=false for an
// empty or malformed token, in which case the caller treats it as "first page".
func decodeKeysetCursor(cursor string) (sortVal, id string, ok bool) {
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

// nullStrParam binds s as a SQL value, or NULL when s is empty — so an unused
// keyset-seek bound param casts cleanly instead of failing on an empty-string
// cast under a short-circuited predicate.
func nullStrParam(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(m map[string]any, key string) *time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case *time.Time:
			return t
		case time.Time:
			return &t
		case string:
			parsed, err := time.Parse(time.RFC3339, t)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

// nullBool reads a bool out of an updates map, returning nil (binds SQL NULL,
// leaving a COALESCE target untouched) when the key is absent or not a bool.
func nullBool(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return nil
}

// boolVal safely dereferences a *bool, returning false for nil.
func boolVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

