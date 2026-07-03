//go:build integration

// rls_schema_matrix_test.go — integration tests that must import crmcore (Tier-1)
// directly. This file matches the *_matrix_test.go glob that is excluded from
// arch-lint (.go-arch-lint.yml → excludeFiles), so it is a test composition root
// exempt from the module DAG rules.
package crmcore_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// TestAuditEventCoverage creates a person via PersonStore.Create and verifies
// that an audit_log row with action='create', entity_type='person' is written.
// The insertAuditLog helper in store.go fires after every successful Create;
// this test proves it is wired end-to-end.
// Uses *sql.DB (lib/pq) because PersonStore uses *sql.DB.
func TestAuditEventCoverage(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create a workspace directly (bypasses RLS; test connection is superuser).
	var wsID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO workspace(name,slug,base_currency) VALUES($1,$2,'EUR') RETURNING id`,
		"audit-cov-"+nonce, "audit-cov-"+nonce).Scan(&wsID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	store := crmcore.NewPersonStore(db)
	// Source/CapturedBy are the persisted provenance columns (the embedded Provenance
	// is internal-only). PersonStore.Create now rejects empty provenance, so set them.
	p, err := store.Create(ctx, crmcore.Person{
		WorkspaceID: wsID,
		FullName:    "Audit Test " + nonce,
		Source:      "api",
		CapturedBy:  "human:test",
		Provenance: prov.Provenance{
			Source:     "api",
			CapturedBy: "human:test",
		},
	})
	if err != nil {
		t.Fatalf("PersonStore.Create: %v", err)
	}

	// Query audit_log for at least 1 row with action='create', entity_type='person'.
	var auditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_log WHERE workspace_id=$1::uuid AND action='create' AND entity_type='person'`,
		wsID).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditCount < 1 {
		t.Errorf("audit_log: expected at least 1 create/person row for workspace %s (person %s), got %d",
			wsID, p.ID, auditCount)
	}
}
