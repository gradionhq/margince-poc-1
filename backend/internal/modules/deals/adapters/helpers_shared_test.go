//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"testing"
)

// seedAppUser seeds an app_user row so that audit_log.on_behalf_of FK is satisfied
// when agent-principal requests invoke crmaudit.Write.
//
// Generic Postgres test helpers (uniq, openTestDB, setRLS, seedWorkspace) live in
// the Tier-0 shared/kernel/pgtest package.
func seedAppUser(t *testing.T, db *sql.DB, id, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO app_user(id,workspace_id,email,display_name,is_agent) VALUES($1::uuid,$2::uuid,$3,'Agent Test',true) ON CONFLICT DO NOTHING`,
		id, wsID, "e03-agent-"+id+"@example.com")
	if err != nil {
		t.Fatal("seed app_user:", err)
	}
}
