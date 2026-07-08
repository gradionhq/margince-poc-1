//go:build integration

package customfields_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// TestRetire_FlipsStatus_PreservesColumnAndData_ArchivedAtStaysNull proves
// CUSTOM-FIELDS-WIRE-4/AC-13: retire is a status flip only — the physical
// column and every value in it survive, archived_at stays null, and
// exactly one audit row lands.
func TestRetire_FlipsStatus_PreservesColumnAndData_ArchivedAtStaysNull(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	created, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object: "person", Label: "Loyalty tier " + time.Now().Format("150405.000000000"),
		Type: customfields.TypeText, Source: "ui", CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Seed a value into the new column before retiring, to prove it survives.
	personID := seedCFPerson(t, db, wsID)
	mustExec(t, db, `UPDATE person SET `+pqIdent(created.ColumnName)+`=$1 WHERE id=$2::uuid`, "gold", personID)

	retired, err := customfields.Retire(ctx, db, created.ID)
	if err != nil {
		t.Fatalf("Retire: %v", err)
	}
	if retired.Status != "retired" {
		t.Fatalf("expected status=retired, got %q", retired.Status)
	}
	if retired.ArchivedAt != nil {
		t.Fatalf("expected archived_at to stay null on retire, got %v", retired.ArchivedAt)
	}

	var colExists bool
	mustQueryScalar(t, db, &colExists, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='person' AND column_name=$1)`, created.ColumnName)
	if !colExists {
		t.Fatalf("retire must never drop the physical column %s", created.ColumnName)
	}
	var storedValue string
	mustQueryScalar(t, db, &storedValue, `SELECT `+pqIdent(created.ColumnName)+` FROM person WHERE id=$1::uuid`, personID)
	if storedValue != "gold" {
		t.Fatalf("retire must preserve existing column data, got %q want %q", storedValue, "gold")
	}

	var auditCount int
	mustQueryScalar(t, db, &auditCount, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action='update' AND entity_type='custom_field'`, created.ID)
	if auditCount != 1 {
		t.Fatalf("expected exactly one 'update' audit row, got %d", auditCount)
	}
}

func TestRetire_NonexistentID_ErrNotFound(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	if _, err := customfields.Retire(ctx, db, "018f3a1b-0000-7000-8000-0000000000ff"); !errors.Is(err, customfields.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// seedCFPerson inserts a minimal person row in wsID, for tests that need a
// real row to write a custom-field value into.
func seedCFPerson(t *testing.T, db *sql.DB, wsID string) string {
	t.Helper()
	id := ids.New()
	mustExec(t, db, `INSERT INTO person (id, workspace_id, full_name, source, captured_by) VALUES ($1::uuid,$2::uuid,'Test Person','ui','human:seed')`, id, wsID)
	return id
}

// pqIdent double-quotes a known-safe, test-only identifier for interpolation
// into a raw test query (never used on request-derived text in production code).
func pqIdent(s string) string { return `"` + s + `"` }
