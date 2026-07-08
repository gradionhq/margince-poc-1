//go:build integration

package customfields_test

import (
	"context"
	"errors"
	"testing"
	"time"

	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// TestRename_UpdatesLabelOnly_ColumnNameStableOneAuditRow proves
// CUSTOM-FIELDS-WIRE-3/AC-13: renaming a field only ever touches the
// catalog label — column_name is byte-identical before/after — and writes
// exactly one audit row.
func TestRename_UpdatesLabelOnly_ColumnNameStableOneAuditRow(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	created, err := customfields.Create(ctx, db, customfields.FieldSpec{
		Object: "deal", Label: "Renewal date " + time.Now().Format("150405.000000000"),
		Type: customfields.TypeDate, Source: "ui", CapturedBy: "human:" + userID,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	renamed, err := customfields.Rename(ctx, db, created.ID, "Renewal date (updated)")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Label != "Renewal date (updated)" {
		t.Fatalf("expected label to update, got %q", renamed.Label)
	}
	if renamed.ColumnName != created.ColumnName {
		t.Fatalf("column_name must be byte-identical across rename: before=%q after=%q", created.ColumnName, renamed.ColumnName)
	}

	var auditCount int
	mustQueryScalar(t, db, &auditCount, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action='update' AND entity_type='custom_field'`, created.ID)
	if auditCount != 1 {
		t.Fatalf("expected exactly one 'update' audit row, got %d", auditCount)
	}
}

// TestRename_NonexistentID_ErrNotFound proves a rename against a
// nonexistent id is a clean not-found, never a panic/500.
func TestRename_NonexistentID_ErrNotFound(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	if _, err := customfields.Rename(ctx, db, "018f3a1b-0000-7000-8000-0000000000ff", "New label"); !errors.Is(err, customfields.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
