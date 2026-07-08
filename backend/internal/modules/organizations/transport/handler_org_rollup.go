package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

func (h *OrganizationHandler) hierarchyRollup(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	p, _ := crmctx.From(r.Context())
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "tree"
	}
	if scope != "tree" && scope != "self" {
		httpkit.JSONValidationError(w, "scope must be tree or self.", []httpkit.FieldError{{Field: "scope", Code: "invalid_enum"}})
		return
	}
	out, err := h.rollupStore.Compute(r.Context(), id, wsID, p.UserID, scope)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	var fxErr *deals.FXRateUnavailableError
	if errors.As(err, &fxErr) {
		jsonFXRateUnavailable(w, fxErr)
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, toWireRollup(out))
}

// hierarchyRollupMoney is the wire Money struct for the hierarchy-rollup response.
type hierarchyRollupMoney struct {
	AmountMinor *int64  `json:"amount_minor,omitempty"`
	Currency    *string `json:"currency,omitempty"`
}

// hierarchyRollupResponse is the wire shape for GET .../hierarchy-rollup (RD-FORM-1).
type hierarchyRollupResponse struct {
	RootID                 string               `json:"root_id"`
	Scope                  string               `json:"scope"`
	WeightedPipeline       hierarchyRollupMoney `json:"weighted_pipeline"`
	ClosedWon              hierarchyRollupMoney `json:"closed_won"`
	ActivityCount30d       int                  `json:"activity_count_30d"`
	AggregatedAccountCount int                  `json:"aggregated_account_count"`
	ComputedAt             time.Time            `json:"computed_at"`
	RestrictedExcluded     []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"restricted_excluded"`
}

func toWireRollup(out records.RollupResult) hierarchyRollupResponse {
	restricted := make([]struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}, len(out.RestrictedExcluded))
	for i, n := range out.RestrictedExcluded {
		restricted[i].ID = n.ID
		restricted[i].DisplayName = n.DisplayName
	}
	return hierarchyRollupResponse{
		RootID:                 out.RootID,
		Scope:                  out.Scope,
		WeightedPipeline:       hierarchyRollupMoney{AmountMinor: &out.WeightedPipelineMinor, Currency: &out.BaseCurrency},
		ClosedWon:              hierarchyRollupMoney{AmountMinor: &out.ClosedWonMinor, Currency: &out.BaseCurrency},
		ActivityCount30d:       out.ActivityCount30d,
		AggregatedAccountCount: out.AggregatedAccountCount,
		ComputedAt:             out.ComputedAt,
		RestrictedExcluded:     restricted,
	}
}

func jsonFXRateUnavailable(w http.ResponseWriter, err *deals.FXRateUnavailableError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": http.StatusUnprocessableEntity,
		"code":   "fx_rate_unavailable",
		"details": map[string]any{
			"currency": err.Currency,
			"as_of":    err.AsOf.Format("2006-01-02"),
		},
	})
}
