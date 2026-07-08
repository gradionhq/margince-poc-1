package transport

import (
	"errors"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

func (h *QuotaHandler) attainment(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	out, err := h.store.Attainment(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if errors.Is(err, records.ErrAttainmentTargetZero) {
		httpkit.JSONProblem(w, http.StatusUnprocessableEntity, "attainment_target_zero")
		return
	}
	if err != nil {
		httpkit.JSONProblem(w, http.StatusUnprocessableEntity, "attainment_computation_failed")
		return
	}
	httpkit.JSONOK(w, out)
}
