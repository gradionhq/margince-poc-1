//go:build integration

package crmauth_test

import (
	"database/sql"
	"os"
	"testing"
)

func mustDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	return dsn
}

func setWorkspaceGUC(t *testing.T, d *sql.DB, workspaceID string) {
	t.Helper()
	if _, err := d.Exec(`SELECT set_config('app.workspace_id', $1, false)`, workspaceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
}
