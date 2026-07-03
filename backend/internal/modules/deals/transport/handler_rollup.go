package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

const codeFXRateUnavailable = "fx_rate_unavailable"

func (h *PipelineHandler) rollup(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	out, err := h.rollupStore.Get(r.Context(), id, wsID, time.Now().UTC())
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, codeNotFound)
		return
	}
	var fxErr *deals.FXRateUnavailableError
	if errors.As(err, &fxErr) {
		jsonFXRateUnavailable(w, fxErr)
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, out)
}

func jsonFXRateUnavailable(w http.ResponseWriter, err *deals.FXRateUnavailableError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{
		fieldStatus: http.StatusUnprocessableEntity,
		fieldCode:   codeFXRateUnavailable,
		fieldDetails: map[string]any{
			"currency": err.Currency,
			"as_of":    err.AsOf.Format("2006-01-02"),
		},
	})
}
