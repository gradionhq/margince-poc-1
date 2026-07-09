package transport

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// computedFields builds RD-T11's formula-field display rows, returning nil when the
// rollup store is unavailable or the caller lacks computed_field:read visibility.
func (h *OrganizationHandler) computedFields(ctx context.Context, wsID, orgID string) ([]domain.ComputedField, error) {
	if h.rollupStore == nil {
		return nil, nil
	}
	principal, _ := crmctx.From(ctx)
	visible, err := h.rollupStore.ComputedFieldsVisible(ctx, wsID, principal)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, nil
	}
	openPipelineMinor, _, err := h.rollupStore.OpenPipelineRollup(ctx, orgID, wsID)
	if err != nil {
		return nil, err
	}
	v := int64(0)
	if openPipelineMinor != nil {
		v = *openPipelineMinor
	}
	notBuilt := "not_yet_built"
	return []domain.ComputedField{
		{
			Key:          "open_pipeline",
			Label:        "Open pipeline",
			Kind:         "currency_minor",
			ValueMinor:   &v,
			FormulaSQL:   "SUM(deal.amount_minor_base) WHERE organization_id = ... AND status = 'open' AND NOT archived",
			Dependencies: []string{"deal.amount_minor", "deal.fx_rate_to_base", "deal.status"},
			Computable:   true,
		},
		{
			Key:          "weighted_pipeline",
			Label:        "Weighted pipeline",
			Kind:         "currency_minor",
			FormulaSQL:   "",
			Dependencies: []string{},
			Computable:   false,
			Reason:       &notBuilt,
		},
		{
			Key:          "customer_age",
			Label:        "Customer age",
			Kind:         "duration_months",
			FormulaSQL:   "",
			Dependencies: []string{},
			Computable:   false,
			Reason:       &notBuilt,
		},
		{
			Key:          "net_revenue_retention",
			Label:        "Net revenue retention",
			Kind:         "percent",
			FormulaSQL:   "",
			Dependencies: []string{},
			Computable:   false,
			Reason:       &notBuilt,
		},
		{
			Key:          "blended_gross_margin",
			Label:        "Blended gross margin",
			Kind:         "percent",
			FormulaSQL:   "",
			Dependencies: []string{},
			Computable:   false,
			Reason:       &notBuilt,
		},
	}, nil
}
