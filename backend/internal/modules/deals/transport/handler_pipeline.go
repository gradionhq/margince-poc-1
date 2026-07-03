package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// PipelineHandler routes /pipelines and /pipelines/{id} requests to the
// PipelineStore. GET (list/read) ships in Task 1; PATCH ships in Task 2.
type PipelineHandler struct{ store *deals.PipelineStore }

// NewPipelineHandler returns a PipelineHandler.
func NewPipelineHandler(store *deals.PipelineStore) *PipelineHandler {
	return &PipelineHandler{store: store}
}

func (h *PipelineHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/pipelines")
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

func (h *PipelineHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	items, next, err := h.store.List(r.Context(), wsID, r.URL.Query().Get("cursor"), queryLimit(r, 20))
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func (h *PipelineHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	pl, err := h.store.Get(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, codeNotFound)
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pl)
}

func (h *PipelineHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	pl, err := h.store.Update(r.Context(), id, wsID, body)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, codeNotFound)
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pl)
}
