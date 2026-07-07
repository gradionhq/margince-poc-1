//go:build integration

package crosscutting_test

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestNoFieldMetadataTable(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow(`
        SELECT count(*) FROM information_schema.tables
        WHERE table_schema = 'public'
          AND (table_name = 'field_metadata' OR table_name LIKE '%_eav%')
    `).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count > 0 {
		t.Errorf("found %d EAV/field_metadata table(s): violates data-model §9 static-schema invariant", count)
	}
}

// TestProvenanceUniversality is the B-E02.9 gate (features/01 §7.3): introspects the live
// migrated schema and asserts that every capture-bearing table carries a complete, NOT-NULL
// provenance pair (source, captured_by) — no captured row can be origin-less.
func TestProvenanceUniversality(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`
        SELECT c.table_name, c.column_name, c.is_nullable
        FROM information_schema.columns c
        JOIN information_schema.tables t
          ON t.table_schema = c.table_schema AND t.table_name = c.table_name
        WHERE c.table_schema = 'public'
          AND t.table_type = 'BASE TABLE'
          AND c.column_name IN ('source', 'captured_by')
        ORDER BY c.table_name, c.column_name
    `)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var cols []schemaProvColumn
	for rows.Next() {
		var table, column, isNullable string
		if err := rows.Scan(&table, &column, &isNullable); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, schemaProvColumn{Table: table, Column: column, Nullable: isNullable == "YES"})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(cols) == 0 {
		t.Fatal("no source/captured_by columns found — schema not migrated, or introspection query is wrong")
	}

	if v := checkSchemaProvenanceUniversality(cols); len(v) > 0 {
		t.Errorf("provenance-universality gate failed — %d origin-less / incomplete-provenance issue(s):\n  %s",
			len(v), strings.Join(v, "\n  "))
	}
}

// schemaProvColumn is one (table, column, nullable) row from information_schema.columns,
// restricted to 'source' / 'captured_by' column names.
type schemaProvColumn struct {
	Table    string
	Column   string // "source" | "captured_by"
	Nullable bool   // is_nullable = 'YES'
}

// schemaDomainSourceTables lists tables whose `source` column is a domain field rather
// than the Tier-0 (source, captured_by) provenance pair. These are exempt from the
// provenance-universality gate.
var schemaDomainSourceTables = map[string]string{
	"person_consent": "source = consent capture channel (000022_consent); domain field, no captured_by",
	"consent_event":  "source = consent capture channel (000022_consent); domain field, no captured_by",
}

// checkSchemaProvenanceUniversality returns a sorted list of violations.
// Empty result means the invariant holds.
func checkSchemaProvenanceUniversality(cols []schemaProvColumn) []string {
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
			if _, allowed := schemaDomainSourceTables[table]; !allowed {
				violations = append(violations, fmt.Sprintf("%s: has source but no captured_by (add captured_by, or allow-list it in schemaDomainSourceTables as a domain-source field)", table))
			}
		}
	}
	sort.Strings(violations)
	return violations
}
