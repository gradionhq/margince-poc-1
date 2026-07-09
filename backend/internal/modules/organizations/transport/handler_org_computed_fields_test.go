package transport

import (
	"context"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func TestOrganizationHandler_ComputedFields_VisibleWithOpenPipeline(t *testing.T) {
	minor := int64(212000)
	h := &OrganizationHandler{
		rollupStore: &fakeRollupStore{
			openPipelineMinor: &minor,
			visible:           true,
		},
	}

	got, err := h.computedFields(crmctx.With(context.Background(), crmctx.Principal{TenantID: "ws-1", UserID: "user-1"}), "ws-1", "org-1")
	if err != nil {
		t.Fatalf("computedFields: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("len(computedFields)=%d want 5", len(got))
	}

	wantKeys := []string{"open_pipeline", "weighted_pipeline", "customer_age", "net_revenue_retention", "blended_gross_margin"}
	for i, wantKey := range wantKeys {
		if got[i].Key != wantKey {
			t.Fatalf("row %d key=%q want %q", i, got[i].Key, wantKey)
		}
	}
	if got[0].ValueMinor == nil || *got[0].ValueMinor != minor {
		t.Fatalf("open_pipeline.value_minor=%v want %d", got[0].ValueMinor, minor)
	}
	if !got[0].Computable {
		t.Fatal("open_pipeline should be computable")
	}
	if got[0].FormulaSQL == "" {
		t.Fatal("open_pipeline formula_sql should be populated")
	}
	if len(got[0].Dependencies) != 3 {
		t.Fatalf("open_pipeline.dependencies=%v want 3 entries", got[0].Dependencies)
	}

	for _, row := range got[1:] {
		if row.Computable {
			t.Fatalf("%s should not be computable", row.Key)
		}
		if row.FormulaSQL != "" {
			t.Fatalf("%s formula_sql=%q want empty", row.Key, row.FormulaSQL)
		}
		if len(row.Dependencies) != 0 {
			t.Fatalf("%s dependencies=%v want empty", row.Key, row.Dependencies)
		}
		if row.Reason == nil || *row.Reason != "not_yet_built" {
			t.Fatalf("%s reason=%v want not_yet_built", row.Key, row.Reason)
		}
	}
}

func TestOrganizationHandler_ComputedFields_VisibleWithNoOpenPipelineFloorsToZero(t *testing.T) {
	h := &OrganizationHandler{
		rollupStore: &fakeRollupStore{
			visible: true,
		},
	}

	got, err := h.computedFields(crmctx.With(context.Background(), crmctx.Principal{TenantID: "ws-1", UserID: "user-1"}), "ws-1", "org-1")
	if err != nil {
		t.Fatalf("computedFields: %v", err)
	}
	if got[0].ValueMinor == nil || *got[0].ValueMinor != 0 {
		t.Fatalf("open_pipeline.value_minor=%v want 0", got[0].ValueMinor)
	}
}

func TestOrganizationHandler_ComputedFields_InvisibleOrNilStoreReturnsNil(t *testing.T) {
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: "ws-1", UserID: "user-1"})

	t.Run("invisible", func(t *testing.T) {
		h := &OrganizationHandler{
			rollupStore: &fakeRollupStore{visible: false},
		}
		got, err := h.computedFields(ctx, "ws-1", "org-1")
		if err != nil {
			t.Fatalf("computedFields: %v", err)
		}
		if got != nil {
			t.Fatalf("computedFields=%v want nil", got)
		}
	})

	t.Run("nil store", func(t *testing.T) {
		var h OrganizationHandler
		got, err := h.computedFields(ctx, "ws-1", "org-1")
		if err != nil {
			t.Fatalf("computedFields: %v", err)
		}
		if got != nil {
			t.Fatalf("computedFields=%v want nil", got)
		}
	})
}
