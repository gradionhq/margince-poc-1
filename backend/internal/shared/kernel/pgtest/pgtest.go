// Package pgtest is the Tier-0 kernel of generic PostgreSQL integration-test
// support helpers shared by every module's `*_test` packages: opening the test
// DB from TEST_DATABASE_URL, engaging row-level security, seeding a throwaway
// workspace, and small assertion/introspection utilities.
//
// Go's `_test.go` files cannot import another package's test-only symbols, so
// before this package every module carried a byte-for-byte copy of these
// helpers (each file's doc comment openly admitted the duplication). This is a
// normal test-support library — like net/http/httptest, it takes *testing.T and
// is imported by (integration-tagged) test files, not run itself.
//
// Only the truly generic, identical helpers live here; module-specific fixtures
// (entity factories, pinned clocks, per-module seeding) stay in each package.
package pgtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq" // register the "postgres" database/sql driver

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

var (
	testSeq      int64
	testRunEpoch = time.Now().UnixNano()
)

// Uniq returns a string unique across test-binary invocations.
func Uniq() string {
	return fmt.Sprintf("%d-%d", testRunEpoch, atomic.AddInt64(&testSeq, 1))
}

// OpenTestDB opens a *sql.DB from TEST_DATABASE_URL and registers a cleanup.
func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// SetRLS sets app.workspace_id on the connection so Postgres RLS applies.
func SetRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), "SET app.workspace_id = '"+wsID+"'"); err != nil {
		t.Fatal("setRLS:", err)
	}
}

// SeedWorkspace inserts a workspace row with the given id, ignoring conflicts.
// The name/slug are cosmetic and derived from wsID so seeding stays collision-free.
func SeedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'pgtest',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "pgtest-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// NewWorkspaceSQL inserts a fresh workspace with an auto-generated UUID and
// returns the new id. For tests needing a pristine, unique workspace per run.
func NewWorkspaceSQL(t *testing.T, db *sql.DB) string {
	t.Helper()
	nonce := fmt.Sprintf("pgtest-%d", time.Now().UnixNano())
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO workspace(name, slug, base_currency) VALUES ($1, $2, 'EUR') RETURNING id`,
		"ws-"+nonce, "slug-"+nonce).Scan(&id)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return id
}

// AppCtx builds a principal-bearing context with a human user for store-layer
// audit writes (EntryFromPrincipal needs UserID+TenantID).
func AppCtx(ws string) context.Context {
	return crmctx.With(context.Background(),
		crmctx.Principal{UserID: "human:store-rls-test", TenantID: ws})
}

// AssertNoRows fails the test if the query returns any rows.
func AssertNoRows(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	var x int
	err := db.QueryRowContext(context.Background(), query, args...).Scan(&x)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no rows for %q, got x=%d err=%v", query, x, err)
	}
}

// FKIntoTable returns every (referencing_table→referencing_column) pair with a
// live FOREIGN KEY into table(id) — the DB-truth version of "grep migrations".
func FKIntoTable(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON tc.constraint_name = ccu.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = $1`, table)
	if err != nil {
		t.Fatalf("fkIntoTable(%s): %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var refTable, refCol string
		if err := rows.Scan(&refTable, &refCol); err != nil {
			t.Fatal(err)
		}
		out[refTable] = refCol
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("fkIntoTable(%s) rows: %v", table, err)
	}
	return out
}
