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

func seedCFWorkspaceAndUser(t *testing.T, db *sql.DB) (wsID, userID string) {
	t.Helper()
	wsID, userID = ids.New(), ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "cf-ws-"+wsID, "cf-ws-"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, userID, wsID, "u"+userID+"@t.test", "U")
	return wsID, userID
}

func TestCreate_Success_ColumnCatalogAndAuditLandTogether(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	spec := customfields.FieldSpec{Object: "deal", Label: "Renewal date " + time.Now().Format("150405.000000000"), Type: customfields.TypeDate, Source: "ui", CapturedBy: "human:" + userID}
	created, err := customfields.Create(ctx, db, spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ColumnName == "" || created.Status != "active" {
		t.Fatalf("unexpected created row: %+v", created)
	}

	var colExists bool
	mustQueryScalar(t, db, &colExists, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='deal' AND column_name=$1)`, created.ColumnName)
	if !colExists {
		t.Fatalf("expected column %s to exist on deal", created.ColumnName)
	}

	var catalogCount, auditCount int
	mustQueryScalar(t, db, &catalogCount, `SELECT count(*) FROM custom_field WHERE id=$1::uuid`, created.ID)
	mustQueryScalar(t, db, &auditCount, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action='create' AND entity_type='custom_field'`, created.ID)
	if catalogCount != 1 || auditCount != 1 {
		t.Fatalf("expected exactly one catalog row and one audit row, got catalog=%d audit=%d", catalogCount, auditCount)
	}
}

// TestCreate_AtomicRollback_OnCatalogConflict proves the three-way atomicity
// (CUSTOM-FIELDS-AC-2/AC-10): the ALTER TABLE runs, but the catalog INSERT
// fails on a pre-seeded (workspace_id, object, slug) collision — the whole
// transaction, including the physical column, must roll back. Postgres DDL
// is transactional, so this is a real proof, not a mock.
func TestCreate_AtomicRollback_OnCatalogConflict(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	label := "Conflict slug " + time.Now().Format("150405.000000000")
	slug := customfields.DeriveSlug(label)
	columnName := customfields.ColumnName(slug)

	// Pre-seed a catalog row claiming this (workspace, object, slug) WITHOUT
	// actually adding the physical column — simulates the collision landing
	// mid-transaction, at the INSERT step, after the ALTER already ran.
	mustExec(t, db, `INSERT INTO custom_field (workspace_id, object, slug, label, type, column_name, created_by)
		VALUES ($1::uuid,'organization',$2,$3,'text',$4,$5::uuid)`, wsID, slug, "pre-existing", columnName, userID)

	spec := customfields.FieldSpec{Object: "organization", Label: label, Type: customfields.TypeText, Source: "ui", CapturedBy: "human:" + userID}
	if _, err := customfields.Create(ctx, db, spec); err == nil {
		t.Fatal("expected Create to fail on the catalog unique-index collision")
	}

	var colExists bool
	mustQueryScalar(t, db, &colExists, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='organization' AND column_name=$1)`, columnName)
	if colExists {
		t.Fatalf("ALTER TABLE must have rolled back with the rest of the transaction — column %s exists", columnName)
	}
	var catalogCount int
	mustQueryScalar(t, db, &catalogCount, `SELECT count(*) FROM custom_field WHERE workspace_id=$1::uuid AND object='organization' AND slug=$2`, wsID, slug)
	if catalogCount != 1 {
		t.Fatalf("expected exactly the one pre-seeded catalog row (no second insert survived), got %d", catalogCount)
	}
}

func TestCreate_StructuralLabelRefused_NothingWritten(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	spec := customfields.FieldSpec{Object: "deal", Label: "New relationship to partner org", Type: customfields.TypeText, Source: "ui", CapturedBy: "human:" + userID}
	if _, err := customfields.Create(ctx, db, spec); err != customfields.ErrStructural {
		t.Fatalf("expected ErrStructural, got %v", err)
	}
	var n int
	mustQueryScalar(t, db, &n, `SELECT count(*) FROM custom_field WHERE workspace_id=$1::uuid`, wsID)
	if n != 0 {
		t.Fatalf("structural refusal must write nothing, got %d catalog rows", n)
	}
}

func TestCreate_ValidationFailure_NothingWritten(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	spec := customfields.FieldSpec{Object: "deal", Label: "Bad type", Type: "money", Source: "ui", CapturedBy: "human:" + userID}
	_, err := customfields.Create(ctx, db, spec)
	var verr *customfields.ErrValidation
	if !errors.As(err, &verr) {
		t.Fatalf("expected *ErrValidation, got %v", err)
	}
	var n int
	mustQueryScalar(t, db, &n, `SELECT count(*) FROM custom_field WHERE workspace_id=$1::uuid`, wsID)
	if n != 0 {
		t.Fatalf("validation failure must write nothing, got %d catalog rows", n)
	}
}
