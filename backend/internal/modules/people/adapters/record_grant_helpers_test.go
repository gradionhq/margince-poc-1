//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// sqlDB opens a *sql.DB against TEST_DATABASE_URL, closing it on test cleanup.
// Shared by every record_grant integration test in this package (moved from
// modules/directory's helpers_shared_external_test.go, GH-81 Task 6 split).
func sqlDB(t *testing.T) *sql.DB {
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

// newWorkspaceSQL inserts a fresh workspace row (unique slug, EUR base
// currency) and returns its id, for tests that only need a valid
// workspace_id to seed fixtures against — same shape as the other split
// modules' seedWorkspace(t, db, wsID) helpers, just returning a
// freshly-generated id instead of taking one as a parameter.
func newWorkspaceSQL(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'record-grant-test',$2,'EUR')`,
		wsID, "record-grant-test-"+wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return wsID
}
