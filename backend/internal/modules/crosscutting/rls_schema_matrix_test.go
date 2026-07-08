//go:build integration

// rls_schema_matrix_test.go — integration tests that span multiple entity modules.
// Lives in crosscutting because it creates persons via the people module store and
// verifies the audit-log side effect — a cross-layer assertion exempt from the module DAG.
package crosscutting_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	people "github.com/gradionhq/margince/backend/internal/modules/people"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// TestAuditEventCoverage creates a person via PersonStore.Create and verifies
// that an audit_log row with action='create', entity_type='person' is written.
func TestAuditEventCoverage(t *testing.T) {
	db := sqlDB(t)

	ctx := context.Background()
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())

	var wsID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO workspace(name,slug,base_currency) VALUES($1,$2,'EUR') RETURNING id`,
		"audit-cov-"+nonce, "audit-cov-"+nonce).Scan(&wsID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	store := people.NewPersonStore(db)
	person := people.NewPerson("Audit Test "+nonce, prov.Provenance{
		Source:     "api",
		CapturedBy: "human:test",
	})
	person.WorkspaceID = wsID
	p, err := store.Create(ctx, person, nil)
	if err != nil {
		t.Fatalf("PersonStore.Create: %v", err)
	}

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
