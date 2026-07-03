//go:build integration

package crmcore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'e03test',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "e03-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// openTestDB, setRLS, and uniq duplicate
// modules/people/transport/handler_person_test.go's helpers of the same name:
// handler_person_test.go moved to package transport in the 1c restructure
// (task-3-brief.md) while handler_audit_history_test.go and
// store_deal_filter_test.go — which also use these — stayed in package
// crmcore (modules/directory); the two packages can no longer share a
// _test.go file. Duplicated rather than exported solely for this — same class
// of directory-move-forced duplication as httpserver's keyStatus (see
// internal/platform/httpserver/middleware.go).
var (
	testSeq      int64
	testRunEpoch = time.Now().UnixNano()
)

// uniq returns a string that is unique across test binary invocations.
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
	_, err := db.ExecContext(context.Background(),
		"SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
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
