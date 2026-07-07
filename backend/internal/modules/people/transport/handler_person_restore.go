// Package transport — PersonHandler.restore, split out of handler_person.go
// to keep it under the T1 500-LOC file cap (architecture/18 §3.2).
package transport

import (
	"errors"
	"net/http"

	peopleadapters "github.com/gradionhq/margince/backend/internal/modules/people/adapters"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// restore implements POST /people/{id}/restore.
func (h *PersonHandler) restore(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	restored, err := h.store.Restore(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if errors.Is(err, errs.ErrNotArchived) {
		jsonProblemDetails(w, http.StatusUnprocessableEntity, "validation_error",
			"person is already live.",
			map[string]any{fieldErrors: []fieldError{{Field: "archived_at", Code: "not_archived"}}})
		return
	}
	if errors.Is(err, errs.ErrMergedRecord) {
		jsonProblemDetails(w, http.StatusUnprocessableEntity, "validation_error",
			"merged_into_id must be cleared before restore.",
			map[string]any{fieldErrors: []fieldError{{Field: "merged_into_id", Code: "merged_record"}}})
		return
	}
	var dup *peopleadapters.ErrDuplicateEmail
	if errors.As(err, &dup) {
		jsonProblemDetails(w, http.StatusConflict, "duplicate_email",
			"An active person already owns this email.",
			map[string]any{"existing_id": dup.ExistingID, "field": dup.Field})
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, restored)
}
