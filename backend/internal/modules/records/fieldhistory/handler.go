package fieldhistory

import (
	"context"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/authz"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

// fieldHistoryStore is the minimal persistence interface satisfied by *Store.
type fieldHistoryStore interface {
	List(ctx context.Context, workspaceID, entityType, entityID string, field, actorType *string, cursor string, limit int) ([]Entry, string, error)
}

var validEntityTypes = map[string]bool{
	"person": true, "organization": true, "deal": true, "lead": true, "activity": true,
}

var validActorTypes = map[string]bool{
	"human": true, "agent": true, "system": true, "connector": true,
}

// Handler serves GET /field-history.
type Handler struct {
	store fieldHistoryStore
	authz authz.Authorizer
}

// NewHandler returns a Handler that uses store and az.
func NewHandler(store fieldHistoryStore, az authz.Authorizer) *Handler {
	return &Handler{store: store, authz: az}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entityType := q.Get("entity_type")
	entityID := q.Get("entity_id")

	if entityType == "" || entityID == "" {
		httpkit.JSONProblem(w, http.StatusBadRequest, "missing_params")
		return
	}
	if !authz.ReUUID.MatchString(entityID) {
		httpkit.JSONProblem(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if !validEntityTypes[entityType] {
		httpkit.JSONProblem(w, http.StatusBadRequest, "invalid_entity_type")
		return
	}

	var actorType *string
	if at := q.Get("actor_type"); at != "" {
		if !validActorTypes[at] {
			httpkit.JSONProblem(w, http.StatusBadRequest, "invalid_actor_type")
			return
		}
		actorType = &at
	}

	cursor := q.Get("cursor")
	if cursor != "" {
		if _, _, ok := decodeCursor(cursor); !ok {
			httpkit.JSONProblem(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
	}

	var field *string
	if f := q.Get("field"); f != "" {
		field = &f
	}

	if err := h.authz(r.Context(), entityType, "read"); err != nil {
		httpkit.JSONProblem(w, http.StatusForbidden, "forbidden")
		return
	}

	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}

	limit := httpkit.QueryLimit(r, defaultFieldHistoryLimit)

	entries, nextCursor, err := h.store.List(r.Context(), wsID, entityType, entityID, field, actorType, cursor, limit)
	if err != nil {
		httpkit.JSONProblem(w, http.StatusInternalServerError, "internal_error")
		return
	}

	httpkit.JSONOK(w, httpkit.PageResponse(entries, nextCursor))
}
