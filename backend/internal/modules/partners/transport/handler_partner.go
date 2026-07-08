// Package transport holds the partners module's HTTP handlers.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/partners/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/partners/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

// PartnerHandler routes /organizations/{id}/partner and /partners requests to
// the PartnerStore.
type PartnerHandler struct{ store *adapters.PartnerStore }

// NewPartnerHandler returns a PartnerHandler.
func NewPartnerHandler(store *adapters.PartnerStore) *PartnerHandler {
	return &PartnerHandler{store: store}
}

func (h *PartnerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch {
	case r.Method == http.MethodPut && id != "":
		h.upsert(w, r, id)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *PartnerHandler) upsert(w http.ResponseWriter, r *http.Request, orgID string) {
	wsID := httpkit.WorkspaceID(r)
	var body struct {
		PartnerRole    *string        `json:"partner_role"`
		CertStatus     string         `json:"cert_status"`
		MarginTier     *string        `json:"margin_tier"`
		GateMetrics    map[string]any `json:"gate_metrics"`
		CertifiedStaff *int           `json:"certified_staff"`
		RetentionRate  *float64       `json:"retention_rate"`
		JoinedAt       *string        `json:"joined_at"`
		RenewsAt       *string        `json:"renews_at"`
		Source         string         `json:"source"`
		CapturedBy     string         `json:"captured_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.Source == "" || body.CapturedBy == "" {
		httpkit.JSONValidationError(w, "source and captured_by are required.",
			[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})

		return
	}

	p := domain.NewPartner(orgID, provenanceOf(body.Source, body.CapturedBy))
	p.WorkspaceID = wsID
	p.PartnerRole = body.PartnerRole
	if body.CertStatus != "" {
		p.CertStatus = body.CertStatus
	}
	p.MarginTier = body.MarginTier
	p.GateMetrics = body.GateMetrics
	if body.CertifiedStaff != nil {
		p.CertifiedStaff = *body.CertifiedStaff
	}
	p.RetentionRate = body.RetentionRate
	if body.JoinedAt != nil {
		t, err := parsePartnerDateField(*body.JoinedAt)
		if err != nil {
			httpkit.JSONProblem(w, http.StatusBadRequest, "bad_joined_at")
			return
		}
		p.JoinedAt = &t
	}
	if body.RenewsAt != nil {
		t, err := parsePartnerDateField(*body.RenewsAt)
		if err != nil {
			httpkit.JSONProblem(w, http.StatusBadRequest, "bad_renews_at")
			return
		}
		p.RenewsAt = &t
	}

	created, err := h.store.Upsert(r.Context(), p)
	if errors.Is(err, errs.ErrNullProvenance) {
		httpkit.JSONValidationError(w, "source and captured_by are required.",
			[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})

		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, created)
}

func (h *PartnerHandler) get(w http.ResponseWriter, r *http.Request, orgID string) {
	wsID := httpkit.WorkspaceID(r)
	p, err := h.store.Get(r.Context(), orgID, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, p)
}

func (h *PartnerHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter := domain.PartnerListFilter{
		PartnerRole: q.Get("partner_role"),
		CertStatus:  q.Get("cert_status"),
	}
	items, next, err := h.store.List(r.Context(), wsID, q.Get("cursor"), httpkit.QueryLimit(r, 20), filter)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, next))
}

func parsePartnerDateField(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
