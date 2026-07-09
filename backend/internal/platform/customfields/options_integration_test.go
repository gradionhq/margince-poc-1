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

// TestSetOptions_RegeneratesCheck_OneAuditRow proves CUSTOM-FIELDS-PARAM-5:
// adding an option lets a row with the new value insert successfully;
// removing an option makes a row with the removed value fail the
// regenerated CHECK.
func TestSetOptions_RegeneratesCheck_OneAuditRow(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	created, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object: "deal", Label: "Procurement route " + time.Now().Format("150405.000000000"),
		Type: customfields.TypePicklist, Options: []string{"direct", "reseller"}, Source: "ui", CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := customfields.SetOptions(ctx, db, created.ID, []string{"direct", "marketplace"})
	if err != nil {
		t.Fatalf("SetOptions: %v", err)
	}
	if len(updated.Options) != 2 || updated.Options[0] != "direct" || updated.Options[1] != "marketplace" {
		t.Fatalf("expected catalog options to be replaced, got %+v", updated.Options)
	}

	// New option accepted by the regenerated CHECK.
	dealID := seedCFDeal(t, db, wsID)
	if _, err := db.Exec(`UPDATE deal SET `+pqIdent(created.ColumnName)+`=$1 WHERE id=$2::uuid`, "marketplace", dealID); err != nil {
		t.Fatalf("expected the new option to satisfy the regenerated CHECK: %v", err)
	}
	// Removed option now rejected by the regenerated CHECK.
	if _, err := db.Exec(`UPDATE deal SET `+pqIdent(created.ColumnName)+`=$1 WHERE id=$2::uuid`, "reseller", dealID); err == nil {
		t.Fatal("expected the removed option 'reseller' to now violate the regenerated CHECK")
	}

	var auditCount int
	mustQueryScalar(t, db, &auditCount, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action='update' AND entity_type='custom_field'`, created.ID)
	if auditCount != 1 {
		t.Fatalf("expected exactly one 'update' audit row, got %d", auditCount)
	}
}

func TestSetOptions_RemovingLastOption_ErrLastOption(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	created, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object: "deal", Label: "Solo option field " + time.Now().Format("150405.000000000"),
		Type: customfields.TypePicklist, Options: []string{"only"}, Source: "ui", CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := customfields.SetOptions(ctx, db, created.ID, nil); !errors.Is(err, customfields.ErrLastOption) {
		t.Fatalf("expected ErrLastOption for an empty options list, got %v", err)
	}
}

func TestSetOptions_NonPicklistField_ErrNotPicklist(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	created, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object: "deal", Label: "Not a picklist " + time.Now().Format("150405.000000000"),
		Type: customfields.TypeText, Source: "ui", CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := customfields.SetOptions(ctx, db, created.ID, []string{"x"}); !errors.Is(err, customfields.ErrNotPicklist) {
		t.Fatalf("expected ErrNotPicklist, got %v", err)
	}
}

func TestSetOptions_NonexistentID_ErrNotFound(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	if _, err := customfields.SetOptions(ctx, db, "018f3a1b-0000-7000-8000-0000000000ff", []string{"x"}); !errors.Is(err, customfields.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// seedCFDeal inserts a minimal deal row in wsID, for tests that need a real
// row to write a custom-field value into. Check
// backend/migrations/000003_core_objects.up.sql's deal table for its NOT
// NULL columns before adjusting this INSERT.
func seedCFDeal(t *testing.T, db *sql.DB, wsID string) string {
	t.Helper()
	id := ids.New()
	pipelineID := ids.New()
	stageID := ids.New()
	mustExec(t, db, `INSERT INTO pipeline (id, workspace_id, name) VALUES ($1::uuid, $2::uuid, 'Test pipeline')`, pipelineID, wsID)
	mustExec(t, db, `INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability) VALUES ($1::uuid, $2::uuid, $3::uuid, 'Test stage', 1, 'open', 50)`, stageID, wsID, pipelineID)
	mustExec(t, db, `INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, source, captured_by) VALUES ($1::uuid,$2::uuid,'Test deal',$3::uuid,$4::uuid,'ui','human:seed')`, id, wsID, pipelineID, stageID)
	return id
}
