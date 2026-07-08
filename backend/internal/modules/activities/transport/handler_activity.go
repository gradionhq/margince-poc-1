// Package transport: this file adds the /activities and /activities/{id}
// HTTP handler, mirroring directory/transport/handler_activity.go.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

// activityStoreSeam is the subset of *adapters.ActivityStore this handler needs.
type activityStoreSeam interface {
	Get(ctx context.Context, id, workspaceID string) (actdomain.Activity, error)
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]actdomain.Activity, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (actdomain.Activity, error)
	Archive(ctx context.Context, id, workspaceID string) (actdomain.Activity, error)
}

// ActivityHandler routes /activities and /activities/{id} requests.
// Scoped to list (timeline), get, patch (task completion / edits), and archive.
// POST /activities is intentionally not wired here — ActivityStore.Create only
// inserts the activity row itself, not the activity_link rows the contract's
// CreateActivityRequest.links field requires, so a correct implementation is a
// materially larger change than this gap-fix task.
type ActivityHandler struct{ store activityStoreSeam }

// NewActivityHandler returns an ActivityHandler backed by the given store.
func NewActivityHandler(store activityStoreSeam) *ActivityHandler {
	return &ActivityHandler{store: store}
}

func (h *ActivityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := httpkit.PathID(r.URL.Path, "/activities")
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
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	items, next, err := h.store.List(r.Context(), wsID, q.Get("entity_type"), q.Get("entity_id"), q.Get("cursor"), httpkit.QueryLimit(r, 20))
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, next))
}

func (h *ActivityHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	a, err := h.store.Get(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, a)
}

func (h *ActivityHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	ifMatch, malformed := httpkit.ParseIfMatch(r)
	if malformed {
		httpkit.JSONProblem(w, http.StatusBadRequest, "bad_if_match")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	a, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	httpkit.WriteUpdateResult(w, a, err)
}

// archive is intentionally If-Match-free, mirroring RelationshipHandler.archive.
func (h *ActivityHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	a, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, a)
}
