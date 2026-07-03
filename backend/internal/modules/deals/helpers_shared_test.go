//go:build integration

package deals

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// uniq, openTestDB, setRLS, and seedWorkspace duplicate
// modules/directory/helpers_shared_test.go's helpers of the same name —
// package deals can't share a _test.go file across the package boundary.
// Same duplication class as that file's own doc comment describes.
var (
	testSeq      int64
	testRunEpoch = time.Now().UnixNano()
)

func uniq() string {
	return fmt.Sprintf("%d-%d", testRunEpoch, atomic.AddInt64(&testSeq, 1))
}

func openTestDB(t *testing.T) *sql.DB {
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

func setRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), "SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
	}
}

func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'t10test',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "t10-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}
