package crmcore_test

import (
	"strings"
	"testing"
)

// These unit tests exercise the pure provenance-universality checker over synthetic
// information_schema rows — no DB, no build tag, so they run under plain `go test`.
// The live-schema assertion (B-E02.9 AC-1/AC-5) lives in schema_guard_test.go behind
// the `integration` build tag and feeds the real introspected rows into the same checker.

// helper: build a provColumn slice tersely.
func provCols(rows ...provColumn) []provColumn { return rows }

func TestProvenanceUniversality_HoldsForWellFormedTables(t *testing.T) {
	// A captured table with both columns NOT NULL is clean.
	cols := provCols(
		provColumn{Table: "person", Column: "source", Nullable: false},
		provColumn{Table: "person", Column: "captured_by", Nullable: false},
		provColumn{Table: "activity", Column: "captured_by", Nullable: false},
		provColumn{Table: "activity", Column: "source", Nullable: false},
	)
	if v := checkProvenanceUniversality(cols); len(v) != 0 {
		t.Errorf("expected no violations, got: %v", v)
	}
}

func TestProvenanceUniversality_CapturedByWithoutSource(t *testing.T) {
	cols := provCols(
		provColumn{Table: "widget", Column: "captured_by", Nullable: false},
	)
	v := checkProvenanceUniversality(cols)
	if len(v) != 1 || !strings.Contains(v[0], "widget") || !strings.Contains(v[0], "source") {
		t.Errorf("expected a violation naming widget/source, got: %v", v)
	}
}

func TestProvenanceUniversality_SourceWithoutCapturedByIsViolation(t *testing.T) {
	// A non-allow-listed table with source but no captured_by is a half-provenance break.
	cols := provCols(
		provColumn{Table: "widget", Column: "source", Nullable: false},
	)
	v := checkProvenanceUniversality(cols)
	if len(v) != 1 || !strings.Contains(v[0], "widget") || !strings.Contains(v[0], "captured_by") {
		t.Errorf("expected a violation naming widget/captured_by, got: %v", v)
	}
}

func TestProvenanceUniversality_NullableColumnsCaught(t *testing.T) {
	cols := provCols(
		provColumn{Table: "person", Column: "source", Nullable: true},
		provColumn{Table: "person", Column: "captured_by", Nullable: true},
	)
	v := checkProvenanceUniversality(cols)
	if len(v) != 2 {
		t.Fatalf("expected 2 nullable violations, got %d: %v", len(v), v)
	}
	joined := strings.Join(v, " | ")
	if !strings.Contains(joined, "person.source") || !strings.Contains(joined, "person.captured_by") {
		t.Errorf("expected violations to name person.source and person.captured_by, got: %v", v)
	}
}

func TestProvenanceUniversality_DomainSourceTablesAllowListed(t *testing.T) {
	// person_consent / consent_event carry `source` (consent channel) with no captured_by —
	// a domain field, allow-listed, must NOT fail the gate.
	cols := provCols(
		provColumn{Table: "person_consent", Column: "source", Nullable: false},
		provColumn{Table: "consent_event", Column: "source", Nullable: false},
	)
	if v := checkProvenanceUniversality(cols); len(v) != 0 {
		t.Errorf("allow-listed domain-source tables must not violate, got: %v", v)
	}
}
