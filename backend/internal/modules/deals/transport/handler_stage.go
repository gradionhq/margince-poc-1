package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// StageHandler routes /stages and /stages/{id} requests to the StageStore.
// GET (list/read) ships in Task 1; PATCH ships in Task 2.
type StageHandler struct{ store *deals.StageStore }

// NewStageHandler returns a StageHandler.
func NewStageHandler(store *deals.StageStore) *StageHandler {
	return &StageHandler{store: store}
}

func (h *StageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/stages")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *StageHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	pipelineID := r.URL.Query().Get("pipeline_id")
	items, next, err := h.store.List(r.Context(), wsID, pipelineID, r.URL.Query().Get("cursor"), queryLimit(r, 50))
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func (h *StageHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	st, err := h.store.Get(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, codeNotFound)
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, st)
}

func (h *StageHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	st, err := h.store.Update(r.Context(), id, wsID, body)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, codeNotFound)
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, st)
}
