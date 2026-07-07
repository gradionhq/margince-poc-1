// Package adapters — shared helpers for the organizations adapters layer.
// These are minimal re-implementations of the directory package's store helpers,
// local to this package so the organizations module has no runtime dependency on
// directory's unexported store.go helpers.
package adapters

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// entity-type constant used in audit log entries.
const entityTypeOrganization = "organization"

// Common field name constants used in event payloads and audit entries.
const (
	fieldOrganizationID = "organization_id"
	fieldMergedIntoID   = "merged_into_id"
)

// requireProvenance rejects an empty source or captured_by with a typed sentinel.
func requireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

func marshalJSON(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(v)
	return b
}

func unmarshalJSON(raw []byte, dst *map[string]any) {
	if raw == nil {
		return
	}
	_ = json.Unmarshal(raw, dst)
}

func nullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

// encodeOffsetCursor/decodeOffsetCursor page an in-memory-sorted list.
// Mirrors directory's store_strength.go implementation exactly.
func encodeOffsetCursor(n int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(n)))
}

func decodeOffsetCursor(cursor string) int {
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

// nullTime reads a *time.Time out of an updates map. Kept for completeness;
// org stores only use nullStr, but helpers.go is the shared utility file.
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
// leaving the column unchanged) when the key is absent or not a bool.
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

// encodeKeysetCursor packs (sortVal, id) into one opaque, URL-safe token for
// non-ID orderings that must seek on a composite (sortCol, id) key.
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
// keyset-seek bound param casts cleanly instead of failing on an empty-string cast.
func nullStrParam(s string) any {
	if s == "" {
		return nil
	}
	return s
}
