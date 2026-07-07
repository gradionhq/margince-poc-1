//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

var (
	relTestSeq      int64
	relTestRunEpoch = time.Now().UnixNano()
)

// uniq returns a string that is unique across test binary invocations and
// across concurrent goroutines within the same invocation.
func uniq() string {
	return fmt.Sprintf("%d-%d", relTestRunEpoch, atomic.AddInt64(&relTestSeq, 1))
}

// sqlDB opens a *sql.DB against TEST_DATABASE_URL, closing it on test cleanup.
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

// openTestDB is an alias for sqlDB provided for consistency with other module
// test helpers.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return sqlDB(t)
}

// setRLS sets the Postgres session-local app.workspace_id GUC.
func setRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), "SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
	}
}

// seedWorkspace inserts a workspace row for wsID (ON CONFLICT DO NOTHING).
func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'rel-adapters-test',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "rel-adapters-test-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// strPtr returns a pointer to s, useful for optional string fields in domain structs.
func strPtr(s string) *string { return &s }

// percentile returns the p-th percentile of d (0–100).
func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(d))
	copy(sorted, d)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
