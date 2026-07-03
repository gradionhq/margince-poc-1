package crmcore_test

import (
	"fmt"
	"sort"
)

// B-E02.9 — provenance-universality gate (no origin-less captured row).
//
// The Tier-0 provenance pair is (source, captured_by) (data-model §1.6). The columns are
// declared NOT NULL on every captured table (B-EP02.4) and the central sink rejects a
// null-provenance write at runtime (B-EP05.2). This is the *gate* that makes "no captured
// field is origin-less" a red-or-green build property (features/01 §7.3): a schema-level
// invariant check that fails the build if any capture-bearing table ever loses the guarantee.
//
// `captured_by` is the unambiguous marker of a provenance-bearing table — `source` alone is
// ambiguous (it also names domain fields like the consent capture channel). So the gate keys
// on `captured_by`, with an explicit allow-list for tables whose `source` is a domain field.

// provColumn is one (table, column, nullable) row from information_schema.columns, restricted
// to the exact provenance column names `source` / `captured_by` (the `source_system` /
// `source_id` natural-key columns are excluded by the exact-name match, not in scope here).
type provColumn struct {
	Table    string
	Column   string // "source" | "captured_by"
	Nullable bool   // is_nullable = 'YES'
}

// domainSourceTables lists tables whose `source` column is a domain field — NOT the Tier-0
// (source, captured_by) provenance pair — so they legitimately carry `source` without
// `captured_by` and are exempt from the universality gate. The single config source for the
// allow-list; any addition needs a documented reason here.
var domainSourceTables = map[string]string{
	"person_consent": "source = consent capture channel (000022_consent); domain field, no captured_by",
	"consent_event":  "source = consent capture channel (000022_consent); domain field, no captured_by",
}

// checkProvenanceUniversality returns a sorted, human-readable list of violations over the
// given provenance columns. Empty result == the invariant holds.
//
//   - A table with `captured_by` (provenance-bearing) MUST also have `source`, and both MUST
//     be NOT NULL.
//   - A table with `source` but no `captured_by` is a half-provenance break UNLESS it is on
//     the domain-source allow-list.
func checkProvenanceUniversality(cols []provColumn) []string {
	type tcols struct {
		hasSource, hasCapturedBy         bool
		sourceNullable, capturedNullable bool
	}
	byTable := map[string]*tcols{}
	for _, c := range cols {
		t := byTable[c.Table]
		if t == nil {
			t = &tcols{}
			byTable[c.Table] = t
		}
		switch c.Column {
		case "source":
			t.hasSource = true
			t.sourceNullable = c.Nullable
		case "captured_by":
			t.hasCapturedBy = true
			t.capturedNullable = c.Nullable
		}
	}

	var violations []string
	for table, t := range byTable {
		switch {
		case t.hasCapturedBy:
			if !t.hasSource {
				violations = append(violations, fmt.Sprintf("%s: has captured_by but no source column (provenance pair incomplete)", table))
				continue
			}
			if t.sourceNullable {
				violations = append(violations, fmt.Sprintf("%s.source is nullable (provenance columns must be NOT NULL)", table))
			}
			if t.capturedNullable {
				violations = append(violations, fmt.Sprintf("%s.captured_by is nullable (provenance columns must be NOT NULL)", table))
			}
		case t.hasSource:
			if _, allowed := domainSourceTables[table]; !allowed {
				violations = append(violations, fmt.Sprintf("%s: has source but no captured_by (add captured_by, or allow-list it in domainSourceTables as a domain-source field)", table))
			}
		}
	}
	sort.Strings(violations)
	return violations
}
