// Package adapters contains the partners module's PostgreSQL storage adapters.
package adapters

import (
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// requireProvenance rejects an empty source or captured_by with a typed sentinel
// (data-model §1.6 provenance). HTTP handlers already reject empties at the edge, but
// non-HTTP callers (import/Datasource/direct store use) must not be able to insert
// source="" or captured_by="" — provenance is a load-bearing invariant, not a nicety.
func requireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

// nullStr reads a *string from an updates map; nil when key is absent or not a string.
func nullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}
