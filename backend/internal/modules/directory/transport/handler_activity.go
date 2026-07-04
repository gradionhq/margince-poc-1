// Package transport: this file adds the /activities and /activities/{id}
// HTTP handler, mirroring handler_relationship.go's method-dispatch style.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// activityStoreSeam is the subset of *directory.ActivityStore this handler needs.
type activityStoreSeam interface {
	Get(ctx context.Context, id, workspaceID string) (directory.Activity, error)
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]directory.Activity, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (directory.Activity, error)
	Archive(ctx context.Context, id, workspaceID string) (directory.Activity, error)
}

// ActivityHandler routes /activities and /activities/{id} requests to the
// ActivityStore. Scoped to what the Deal 360 screen (T22) calls today: list
// (timeline), get, and patch (task completion / edits). POST /activities is
// intentionally not wired here — ActivityStore.Create only inserts the
// activity row itself, not the activity_link rows the contract's
// CreateActivityRequest.links field requires, so a correct implementation is
// a materially larger change than this gap-fix task (see task brief: "skip
// with a clear one-line comment ... if it would meaningfully balloon
// scope"). DELETE (archive) is included since ActivityStore.Archive already
// exists and the wiring is mechanical, same as RelationshipHandler.archive.
type ActivityHandler struct{ store activityStoreSeam }

// NewActivityHandler returns an ActivityHandler.
func NewActivityHandler(store *directory.ActivityStore) *ActivityHandler {
	return &ActivityHandler{store: store}
}

func (h *ActivityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/activities")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		h.archive(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (h *ActivityHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	items, next, err := h.store.List(r.Context(), wsID, q.Get("entity_type"), q.Get("entity_id"), q.Get("cursor"), queryLimit(r))
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func (h *ActivityHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	a, err := h.store.Get(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, a)
}

func (h *ActivityHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	ifMatch, malformed := parseIfMatch(r)
	if malformed {
		jsonProblem(w, http.StatusBadRequest, "bad_if_match")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	a, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	writeUpdateResult(w, a, err)
}

// archive is intentionally If-Match-free, mirroring RelationshipHandler.archive.
func (h *ActivityHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	a, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, a)
}
