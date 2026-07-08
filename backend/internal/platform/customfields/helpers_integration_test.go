//go:build integration

package customfields_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver
)

// defaultTestDatabaseDSN is the fallback connection string used when
// TEST_DATABASE_URL isn't set in the environment running these tests.
const defaultTestDatabaseDSN = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"

func testDatabaseDSN() string {
	if dsn, ok := os.LookupEnv("TEST_DATABASE_URL"); ok && dsn != "" {
		return dsn
	}
	return defaultTestDatabaseDSN
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, openErr := sql.Open("postgres", testDatabaseDSN())
	if openErr != nil {
		t.Fatalf("open postgres connection: %v", openErr)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, execErr := db.Exec(query, args...)
	if execErr != nil {
		t.Fatalf("exec failed for %q: %v", query, execErr)
	}
}

func mustQueryScalar(t *testing.T, db *sql.DB, dst any, q string, args ...any) {
	t.Helper()
	if err := db.QueryRow(q, args...).Scan(dst); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
}
