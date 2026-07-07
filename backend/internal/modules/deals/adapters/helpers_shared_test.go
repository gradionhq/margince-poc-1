//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// uniq, openTestDB, setRLS, seedWorkspace, and seedAppUser duplicate
// modules/deals/helpers_shared_test.go's helpers of the same name —
// the parent deals package can't share a _test.go file across the package
// boundary. Same duplication class as that file's own doc comment describes.
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

// seedAppUser seeds an app_user row so that audit_log.on_behalf_of FK is satisfied
// when agent-principal requests invoke crmaudit.Write.
func seedAppUser(t *testing.T, db *sql.DB, id, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO app_user(id,workspace_id,email,display_name,is_agent) VALUES($1::uuid,$2::uuid,$3,'Agent Test',true) ON CONFLICT DO NOTHING`,
		id, wsID, "e03-agent-"+id+"@example.com")
	if err != nil {
		t.Fatal("seed app_user:", err)
	}
}
