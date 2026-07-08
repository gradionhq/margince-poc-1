package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

type offerTemplateStoreSeam interface {
	Create(ctx context.Context, t domain.OfferTemplate) (domain.OfferTemplate, error)
	Get(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.OfferTemplate, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.OfferTemplate, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error)
}

// OfferTemplateHandler routes /offer-templates and /offer-templates/{id} requests.
type OfferTemplateHandler struct{ store offerTemplateStoreSeam }

// NewOfferTemplateHandler returns an OfferTemplateHandler backed by the given store.
func NewOfferTemplateHandler(store offerTemplateStoreSeam) *OfferTemplateHandler {
	return &OfferTemplateHandler{store: store}
}

func (h *OfferTemplateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dispatchCRUD(w, r, "/offer-templates", h.list, h.create, h.get, h.update, h.archive)
}

type createOfferTemplateBody struct {
	Name       string                 `json:"name"`
	Locale     *string                `json:"locale"`
	IsDefault  *bool                  `json:"is_default"`
	Layout     map[string]interface{} `json:"layout"`
	Source     string                 `json:"source"`
	CapturedBy string                 `json:"captured_by"`
}

func (h *OfferTemplateHandler) writeConflict(w http.ResponseWriter, err error) bool {
	var dupName *adapters.ErrDuplicateTemplateName
	if errors.As(err, &dupName) {
		httpkit.JSONProblemDetails(w, http.StatusConflict, "offer_template_name_duplicate",
			"An offer template with this name already exists.",
			map[string]any{fieldExistingID: dupName.ExistingID})
		return true
	}
	var conflict *adapters.ErrDefaultConflict
	if errors.As(err, &conflict) {
		httpkit.JSONProblemDetails(w, http.StatusConflict, "offer_template_default_conflict",
			"A default template already exists for this locale.",
			map[string]any{fieldExistingID: conflict.ExistingID, "locale": conflict.Locale})
		return true
	}
	return false
}

func (h *OfferTemplateHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := httpkit.WorkspaceID(r)
	var body createOfferTemplateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.Source == "" || body.CapturedBy == "" {
		httpkit.JSONValidationError(w, "source and captured_by are required.",
			[]httpkit.FieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
		return
	}
	t := domain.NewOfferTemplate(body.Name, prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy})
	t.WorkspaceID = wsID
	if body.Locale != nil {
		t.Locale = *body.Locale
	}
	if body.IsDefault != nil {
		t.IsDefault = *body.IsDefault
	}
	t.Layout = body.Layout

	created, err := h.store.Create(r.Context(), t)
	if err != nil {
		if h.writeConflict(w, err) {
			return
		}
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONCreatedAt(w, created, "/offer-templates/"+created.ID)
}

func (h *OfferTemplateHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	t, err := h.store.Get(r.Context(), id, httpkit.WorkspaceID(r))
	writeGetResult(w, t, err)
}

func (h *OfferTemplateHandler) list(w http.ResponseWriter, r *http.Request) {
	listResults(w, r, h.store.List)
}

func (h *OfferTemplateHandler) update(w http.ResponseWriter, r *http.Request, id string) {
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
	t, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	if err != nil && h.writeConflict(w, err) {
		return
	}
	httpkit.WriteUpdateResult(w, t, err)
}

func (h *OfferTemplateHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	t, err := h.store.Archive(r.Context(), id, httpkit.WorkspaceID(r))
	writeGetResult(w, t, err)
}
