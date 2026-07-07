// Package adapters — shared helpers for the organizations adapters layer.
// These are minimal re-implementations of the directory package's store helpers,
// local to this package so the organizations module has no runtime dependency on
// directory's unexported store.go helpers.
package adapters

import (
	"encoding/base64"
	"encoding/json"
	"strconv"

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
