//go:build integration

package customfields_test

import (
	"context"
	"testing"
	"time"

	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func TestActiveColumns_ReadsOnlyActiveRowsPerObject(t *testing.T) {
	db := testDB(t)
	wsID, userID := seedCFWorkspaceAndUser(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})
	tag := time.Now().Format("150405.000000000")

	dealText, err := customfields.Create(ctx, db, customfields.FieldSpec{Object: "deal", Label: "Deal text " + tag, Type: customfields.TypeText, Source: "ui", CapturedBy: "human:" + userID})
	if err != nil {
		t.Fatalf("Create deal text: %v", err)
	}
	dealCurrency, err := customfields.Create(ctx, db, customfields.FieldSpec{Object: "deal", Label: "Deal currency " + tag, Type: customfields.TypeCurrency, Currency: "EUR", Source: "ui", CapturedBy: "human:" + userID})
	if err != nil {
		t.Fatalf("Create deal currency: %v", err)
	}
	personText, err := customfields.Create(ctx, db, customfields.FieldSpec{Object: "person", Label: "Person text " + tag, Type: customfields.TypeText, Source: "ui", CapturedBy: "human:" + userID})
	if err != nil {
		t.Fatalf("Create person text: %v", err)
	}

	if _, err := customfields.Retire(ctx, db, dealCurrency.ID); err != nil {
		t.Fatalf("Retire deal currency: %v", err)
	}

	dealCols, err := customfields.ActiveColumns(ctx, db, wsID, "deal")
	if err != nil {
		t.Fatalf("ActiveColumns deal: %v", err)
	}
	if len(dealCols) != 1 {
		t.Fatalf("expected 1 active deal column, got %d: %+v", len(dealCols), dealCols)
	}
	if dealCols[0].ColumnName != dealText.ColumnName || dealCols[0].Slug != dealText.Slug || dealCols[0].Type != dealText.Type {
		t.Fatalf("unexpected deal column: got %+v want %+v", dealCols[0], dealText)
	}

	personCols, err := customfields.ActiveColumns(ctx, db, wsID, "person")
	if err != nil {
		t.Fatalf("ActiveColumns person: %v", err)
	}
	if len(personCols) != 1 {
		t.Fatalf("expected 1 active person column, got %d: %+v", len(personCols), personCols)
	}
	if personCols[0].ColumnName != personText.ColumnName || personCols[0].Slug != personText.Slug || personCols[0].Type != personText.Type {
		t.Fatalf("unexpected person column: got %+v want %+v", personCols[0], personText)
	}
}

func TestActiveColumns_EmptySliceWhenNoRows(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "cf-empty-"+wsID, "cf-empty-"+wsID)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})

	got, err := customfields.ActiveColumns(ctx, db, wsID, "deal")
	if err != nil {
		t.Fatalf("ActiveColumns: %v", err)
	}
	if got == nil {
		return
	}
	if len(got) != 0 {
		t.Fatalf("expected no rows, got %+v", got)
	}
}
