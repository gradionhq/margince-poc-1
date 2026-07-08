//go:build integration

package crosscutting_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// refDB opens a dedicated *sql.DB for referential integrity tests to avoid
// clobbering the shared sqlDB/mustDB helpers already registered in helpers.
func refDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// TestFKColumnsAreIndexed asserts that every FK column in the public schema
// has an index (Postgres does not auto-create them).
func TestFKColumnsAreIndexed(t *testing.T) {
	d := refDB(t)
	rows, err := d.Query(`
        SELECT tc.table_name, kcu.column_name
        FROM information_schema.table_constraints tc
        JOIN information_schema.key_column_usage kcu
          ON tc.constraint_name = kcu.constraint_name
         AND tc.table_schema = kcu.table_schema
        JOIN information_schema.referential_constraints rc
          ON tc.constraint_name = rc.constraint_name
        WHERE tc.table_schema = 'public'
          AND tc.constraint_type = 'FOREIGN KEY'
          AND tc.table_name NOT LIKE 'river_%'
    `)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type col struct{ table, column string }
	var fks []col
	for rows.Next() {
		var c col
		if err := rows.Scan(&c.table, &c.column); err != nil {
			t.Fatal(err)
		}
		fks = append(fks, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	indexQ := `
        SELECT 1 FROM pg_indexes
        WHERE tablename = $1
          AND indexdef LIKE '%(' || $2 || ')%'
        LIMIT 1
    `
	for _, fk := range fks {
		var exists int
		_ = d.QueryRow(indexQ, fk.table, fk.column).Scan(&exists)
		if exists == 0 {
			t.Errorf("FK column %s.%s has no index", fk.table, fk.column)
		}
	}
}

// TestNoCrossCurrencySum asserts that no view sums amount_minor across currencies
// (AC-DS-FX1).
func TestNoCrossCurrencySum(t *testing.T) {
	d := refDB(t)
	var count int
	err := d.QueryRow(`
        SELECT count(*) FROM information_schema.views
        WHERE table_schema = 'public'
          AND view_definition ILIKE '%sum(amount_minor)%'
    `).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count > 0 {
		t.Errorf("found view summing amount_minor across currencies (AC-DS-FX1 violation)")
	}
}
