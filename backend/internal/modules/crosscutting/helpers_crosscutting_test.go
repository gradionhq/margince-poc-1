//go:build integration

// Package crosscutting_test houses integration tests that span multiple entity
// modules and cannot live in any single module without creating import cycles or
// artificial coupling: RBAC matrices, RLS conformance, referential-integrity gates,
// schema structure assertions, and the HTTP conformance "web + server" gate.
package crosscutting_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
)

// mustDB opens a *sql.DB (lib/pq driver). Used by tests that need database/sql.
func mustDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// sqlDB is an alias for mustDB using the postgres driver; used by schema tests.
func sqlDB(t *testing.T) *sql.DB { return mustDB(t) }

// mustPool opens a *pgxpool.Pool (pgx driver). Used by tests that need pgx.
func mustPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return pool
}

// newWorkspaceSQL creates a workspace using *sql.DB and returns its ID.
func newWorkspaceSQL(t *testing.T, db *sql.DB) string {
	t.Helper()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO workspace(name, slug, base_currency) VALUES ($1, $2, 'EUR') RETURNING id`,
		"ws-"+nonce, "slug-"+nonce).Scan(&id)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return id
}
