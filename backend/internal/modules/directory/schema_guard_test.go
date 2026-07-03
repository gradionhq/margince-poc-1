//go:build integration

package crmcore_test

import (
	"database/sql"
	"os"
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
// provenance pair (source, captured_by) — no captured row can be origin-less. The columns +
// NOT-NULL DDL ship in B-EP02.4 and the runtime sink guard in B-EP05.2; this is the build-time
// invariant that keeps the guarantee from silently regressing. The pure checker (and the
// domain-source allow-list) live in provenance_guard_helper_test.go and are unit-tested
// without a DB; here we feed it the real information_schema rows.
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

	// Exact column names only (not LIKE '%source%') so the source_system / source_id
	// natural-key columns are excluded by construction; base tables only (skip views).
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

	var cols []provColumn
	for rows.Next() {
		var table, column, isNullable string
		if err := rows.Scan(&table, &column, &isNullable); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, provColumn{Table: table, Column: column, Nullable: isNullable == "YES"})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(cols) == 0 {
		t.Fatal("no source/captured_by columns found — schema not migrated, or introspection query is wrong")
	}

	if v := checkProvenanceUniversality(cols); len(v) > 0 {
		t.Errorf("provenance-universality gate failed — %d origin-less / incomplete-provenance issue(s):\n  %s",
			len(v), strings.Join(v, "\n  "))
	}
}
