//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// testSeq / testRunEpoch are used by uniq() to generate collision-free names
// across concurrent test runs in the same binary invocation.
// (Duplicated from modules/deals/helpers_shared_test.go — same forced-split
// pattern; tests cannot share _test.go helpers across package boundaries.)
var (
	testSeq      int64
	testRunEpoch = time.Now().UnixNano()
)

// uniq returns a string that is unique across test binary invocations and
// across concurrent goroutines within the same invocation.
func uniq() string {
	return fmt.Sprintf("%d-%d", testRunEpoch, atomic.AddInt64(&testSeq, 1))
}

// openTestDB opens a *sql.DB against TEST_DATABASE_URL and registers a
// test-cleanup close. It delegates to sqlDB (defined in
// record_grant_helpers_test.go) to avoid duplicating the open logic.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return sqlDB(t)
}

// setRLS sets the Postgres session-local app.workspace_id GUC so that RLS
// policies on all tables scope subsequent queries to wsID.
func setRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), "SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
	}
}

// seedWorkspace inserts a workspace row for wsID (ON CONFLICT DO NOTHING) so
// the ID is safe to reference in test fixtures.
func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'adapters-test',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "adapters-test-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// assertNoRows fails the test if query returns any rows. Used by merge tests
// to prove FK rows were relinked off the loser.
func assertNoRows(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	var x int
	err := db.QueryRow(query, args...).Scan(&x)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no rows for %q, got x=%d err=%v", query, x, err)
	}
}
