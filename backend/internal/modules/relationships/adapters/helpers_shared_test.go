//go:build integration

package adapters_test

import (
	"sort"
	"time"
)

// Generic Postgres test helpers (uniq, openTestDB/sqlDB, setRLS, seedWorkspace)
// live in the Tier-0 shared/kernel/pgtest package.

// strPtr returns a pointer to s, useful for optional string fields in domain structs.
func strPtr(s string) *string { return &s }

// percentile returns the p-th percentile of d (0–100).
func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(d))
	copy(sorted, d)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
