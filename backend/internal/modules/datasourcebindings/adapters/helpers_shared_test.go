//go:build integration

package adapters_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// The generic fresh-workspace helper lives in the Tier-0 shared/kernel/pgtest
// package; mustSQLDB is kept local because it pings the DB before returning.
func mustSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
