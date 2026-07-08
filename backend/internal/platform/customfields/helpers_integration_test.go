//go:build integration

package customfields_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func mustQueryScalar(t *testing.T, db *sql.DB, dst any, q string, args ...any) {
	t.Helper()
	if err := db.QueryRow(q, args...).Scan(dst); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
}
