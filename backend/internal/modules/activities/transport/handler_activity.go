// Package transport: this file adds the /activities and /activities/{id}
// HTTP handler, mirroring directory/transport/handler_activity.go.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

// activityStoreSeam is the subset of *adapters.ActivityStore this handler needs.
type activityStoreSeam interface {
	Create(ctx context.Context, a actdomain.Activity) (actdomain.Activity, bool, error)
	Get(ctx context.Context, id, workspaceID string) (actdomain.Activity, error)
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]actdomain.Activity, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (actdomain.Activity, error)
	Archive(ctx context.Context, id, workspaceID string) (actdomain.Activity, error)
}

// ActivityHandler routes /activities and /activities/{id} requests: list
// (timeline), create (logActivity), get, patch (task completion / edits), and
// archive.
type ActivityHandler struct{ store activityStoreSeam }

// NewActivityHandler returns an ActivityHandler backed by the given store.
func NewActivityHandler(store activityStoreSeam) *ActivityHandler {
	return &ActivityHandler{store: store}
}

func (h *ActivityHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind            string     `json:"kind"`
		Subject         *string    `json:"subject,omitempty"`
		Body            *string    `json:"body,omitempty"`
		OccurredAt      *time.Time `json:"occurred_at,omitempty"`
		DueAt           *time.Time `json:"due_at,omitempty"`
		AssigneeID      *string    `json:"assignee_id,omitempty"`
		RemindAt        *time.Time `json:"remind_at,omitempty"`
		DurationSeconds *int       `json:"duration_seconds,omitempty"`
		Direction       *string    `json:"direction,omitempty"`
		MeetingStatus   *string    `json:"meeting_status,omitempty"`
		SourceSystem    *string    `json:"source_system,omitempty"`
		SourceID        *string    `json:"source_id,omitempty"`
		Links           []struct {
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
		} `json:"links,omitempty"`
		Source     string         `json:"source"`
		CapturedBy string         `json:"captured_by"`
		Raw        map[string]any `json:"raw,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}

	var ferrs []fieldError
	if body.Kind == "" {
		ferrs = append(ferrs, fieldError{Field: fieldKind, Code: codeRequired})
	}
	if body.Source == "" {
		ferrs = append(ferrs, fieldError{Field: fieldSource, Code: codeRequired})
	}
	if body.CapturedBy == "" {
		ferrs = append(ferrs, fieldError{Field: fieldCapturedBy, Code: codeRequired})
	}
	if len(ferrs) > 0 {
		httpkit.JSONValidationError(w, "kind, source, and captured_by are required.", ferrs)
		return
	}

	links := make([]actdomain.ActivityLink, 0, len(body.Links))
	for _, l := range body.Links {
		if !validLinkEntityTypes[l.EntityType] {
			httpkit.JSONValidationError(w, "links[].entity_type must be person, organization, or deal.",
				[]fieldError{{Field: "links", Code: "invalid_entity_type"}})
			return
		}
		links = append(links, actdomain.ActivityLink{EntityType: l.EntityType, EntityID: l.EntityID})
	}

	occurredAt := time.Now().UTC()
	if body.OccurredAt != nil {
		occurredAt = *body.OccurredAt
	}
	a := actdomain.Activity{
		WorkspaceID: wsID, Kind: body.Kind, Subject: body.Subject, Body: body.Body,
		OccurredAt: occurredAt, DueAt: body.DueAt, AssigneeID: body.AssigneeID, RemindAt: body.RemindAt,
		DurationSeconds: body.DurationSeconds, Direction: body.Direction, MeetingStatus: body.MeetingStatus,
		SourceSystem: body.SourceSystem, SourceID: body.SourceID,
		Source: body.Source, CapturedBy: body.CapturedBy, Links: links, Raw: body.Raw,
	}

	created, isNew, err := h.store.Create(r.Context(), a)
	if errors.Is(err, errs.ErrNullProvenance) {
		httpkit.JSONValidationError(w, "source and captured_by are required.",
			[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
		return
	}
	if errors.Is(err, errs.ErrFieldNotValidForKind) {
		httpkit.JSONProblem(w, http.StatusUnprocessableEntity, codeFieldNotValidForKind)
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	if isNew {
		httpkit.JSONCreatedAt(w, created, "/activities/"+created.ID)
		return
	}
	httpkit.JSONOK(w, created)
}

func (h *ActivityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := httpkit.PathID(r.URL.Path, "/activities")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
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
	if errors.Is(err, errs.ErrFieldNotValidForKind) {
		httpkit.JSONProblem(w, http.StatusUnprocessableEntity, codeFieldNotValidForKind)
		return
	}
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
