//go:build integration

package crmaudit_test

import (
	"database/sql"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedAuditRow inserts a workspace + one audit_log row and returns the audit id.
func seedAuditRow(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,$4)`,
		wsID, "ao-"+wsID, "ao-"+wsID, "EUR",
	); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	auditID := ids.New()
	if _, err := db.Exec(`
		INSERT INTO audit_log (id, workspace_id, actor_type, actor_id, action, entity_type)
		VALUES ($1::uuid,$2::uuid,'system','system','create','person')`,
		auditID, wsID); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	return auditID
}

func TestAuditLogUpdateFailsLoudly(t *testing.T) {
	db := testDB(t)
	auditID := seedAuditRow(t, db)
	_, err := db.Exec(`UPDATE audit_log SET action='update' WHERE id=$1::uuid`, auditID)
	if err == nil {
		t.Fatal("expected UPDATE on audit_log to RAISE (append-only), got nil error — silent no-op is forbidden")
	}
	// row unchanged
	var action string
	if err := db.QueryRow(`SELECT action FROM audit_log WHERE id=$1::uuid`, auditID).Scan(&action); err != nil {
		t.Fatalf("reselect: %v", err)
	}
	if action != "create" {
		t.Fatalf("row mutated despite append-only: action=%q", action)
	}
}

func TestAuditLogDeleteFailsLoudly(t *testing.T) {
	db := testDB(t)
	auditID := seedAuditRow(t, db)
	_, err := db.Exec(`DELETE FROM audit_log WHERE id=$1::uuid`, auditID)
	if err == nil {
		t.Fatal("expected DELETE on audit_log to RAISE (append-only), got nil error — silent no-op is forbidden")
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE id=$1::uuid`, auditID).Scan(&n); err != nil {
		t.Fatalf("recount: %v", err)
	}
	if n != 1 {
		t.Fatalf("row deleted despite append-only: count=%d", n)
	}
}

func TestAuditLogIsRLSForced(t *testing.T) {
	db := testDB(t)
	// data-model §1: audit_log carries FORCE ROW LEVEL SECURITY (the table is
	// NOT excluded — confirm it IS rls-forced per the migration 000003 loop).
	var relforce bool
	if err := db.QueryRow(
		`SELECT relforcerowsecurity FROM pg_class WHERE relname='audit_log'`,
	).Scan(&relforce); err != nil {
		t.Fatalf("pg_class: %v", err)
	}
	if !relforce {
		t.Fatal("audit_log must have FORCE ROW LEVEL SECURITY per data-model §1")
	}
}
