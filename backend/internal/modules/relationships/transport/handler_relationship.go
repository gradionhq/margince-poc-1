// Package transport holds the relationships module's HTTP handler for
// /relationships and /relationships/{id}.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/relationships/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

const (
	relKindEmployment      = "employment"
	relKindDealStakeholder = "deal_stakeholder"

	// fieldKind/fieldPersonID are defined locally here rather than reused
	// across the package boundary.
	fieldKind     = "kind"
	fieldPersonID = "person_id"
)

type relationshipStoreSeam interface {
	Create(ctx context.Context, rel domain.Relationship) (domain.Relationship, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, filter domain.RelationshipListFilter) ([]domain.Relationship, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Relationship, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Relationship, error)
}

// RelationshipHandler routes /relationships and /relationships/{id} requests
// to the RelationshipStore.
type RelationshipHandler struct{ store relationshipStoreSeam }

// NewRelationshipHandler returns a RelationshipHandler.
func NewRelationshipHandler(store *adapters.RelationshipStore) *RelationshipHandler {
	return &RelationshipHandler{store: store}
}

func (h *RelationshipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/relationships")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	case r.Method == http.MethodPatch && id != "":
		h.update(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		h.archive(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

type createRelationshipBody struct {
	Kind              string  `json:"kind"`
	PersonID          *string `json:"person_id"`
	OrganizationID    *string `json:"organization_id"`
	DealID            *string `json:"deal_id"`
	CounterpartyOrgID *string `json:"counterparty_org_id"`
	Role              *string `json:"role"`
	IsCurrentPrimary  *bool   `json:"is_current_primary"`
	StartedAt         *string `json:"started_at"`
	EndedAt           *string `json:"ended_at"`
	Source            string  `json:"source"`
	CapturedBy        string  `json:"captured_by"`
}

func (b createRelationshipBody) validate() (string, fieldError, bool) {
	switch b.Kind {
	case relKindEmployment:
		if b.PersonID == nil || b.OrganizationID == nil {
			return "employment requires person_id and organization_id.", fieldError{Field: fieldPersonID, Code: codeRequired}, true
		}
	case relKindDealStakeholder:
		if b.PersonID == nil || b.DealID == nil || b.Role == nil || *b.Role == "" {
			return "deal_stakeholder requires deal_id, person_id, and role.", fieldError{Field: "role", Code: codeRequired}, true
		}
	default:
		return "kind must be employment or deal_stakeholder; partner edge kinds are read-only here.", fieldError{Field: fieldKind, Code: codeValidation}, true
	}
	return "", fieldError{}, false
}

func (h *RelationshipHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	var body createRelationshipBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if detail, fe, bad := body.validate(); bad {
		jsonValidationError(w, detail, []fieldError{fe})
		return
	}
	if body.Source == "" || body.CapturedBy == "" {
		jsonValidationError(w, "source and captured_by are required.",
			[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
		return
	}

	rel := domain.Relationship{
		WorkspaceID:       wsID,
		Kind:              body.Kind,
		PersonID:          body.PersonID,
		OrganizationID:    body.OrganizationID,
		DealID:            body.DealID,
		CounterpartyOrgID: body.CounterpartyOrgID,
		Role:              body.Role,
		IsCurrentPrimary:  boolFromPtr(body.IsCurrentPrimary),
		Source:            body.Source,
		CapturedBy:        body.CapturedBy,
	}
	if t, ok, err := parseDateField(body.StartedAt); err != nil {
		jsonProblem(w, http.StatusBadRequest, "bad_started_at")
		return
	} else if ok {
		rel.StartedAt = &t
	}
	if t, ok, err := parseDateField(body.EndedAt); err != nil {
		jsonProblem(w, http.StatusBadRequest, "bad_ended_at")
		return
	} else if ok {
		rel.EndedAt = &t
	}

	created, err := h.store.Create(r.Context(), rel)
	if errors.Is(err, errs.ErrConflict) {
		jsonProblem(w, http.StatusConflict, "conflict")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonCreatedAt(w, created, "/relationships/"+created.ID)
}

func (h *RelationshipHandler) update(w http.ResponseWriter, r *http.Request, id string) {
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
	rel, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	writeUpdateResult(w, rel, err)
}

// archive is intentionally If-Match-free. The contract op carries no version
// header and the store treats an already-archived row as a no-op.
func (h *RelationshipHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	rel, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, rel)
}

func (h *RelationshipHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	includeArchived, _ := strconv.ParseBool(q.Get("include_archived"))
	items, next, err := h.store.List(r.Context(), wsID, q.Get("cursor"), queryLimit(r), domain.RelationshipListFilter{
		Kind:            q.Get("kind"),
		PersonID:        q.Get("person_id"),
		OrganizationID:  q.Get("organization_id"),
		DealID:          q.Get("deal_id"),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func parseDateField(raw *string) (time.Time, bool, error) {
	if raw == nil {
		return time.Time{}, false, nil
	}
	t, err := time.Parse("2006-01-02", *raw)
	return t, err == nil, err
}

func boolFromPtr(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
