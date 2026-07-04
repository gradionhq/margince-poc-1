package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// PartnerHandler routes /organizations/{id}/partner and /partners requests to
// the PartnerStore.
type PartnerHandler struct{ store *directory.PartnerStore }

// NewPartnerHandler returns a PartnerHandler.
func NewPartnerHandler(store *directory.PartnerStore) *PartnerHandler {
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
	wsID := workspaceID(r)
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
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.Source == "" || body.CapturedBy == "" {
		jsonValidationError(w, "source and captured_by are required.",
			[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
		return
	}

	p := directory.NewPartner(orgID, provenanceOf(body.Source, body.CapturedBy))
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
			jsonProblem(w, http.StatusBadRequest, "bad_joined_at")
			return
		}
		p.JoinedAt = &t
	}
	if body.RenewsAt != nil {
		t, err := parsePartnerDateField(*body.RenewsAt)
		if err != nil {
			jsonProblem(w, http.StatusBadRequest, "bad_renews_at")
			return
		}
		p.RenewsAt = &t
	}

	created, err := h.store.Upsert(r.Context(), p)
	if errors.Is(err, errs.ErrNullProvenance) {
		jsonValidationError(w, "source and captured_by are required.",
			[]fieldError{{Field: fieldSource, Code: codeRequired}, {Field: fieldCapturedBy, Code: codeRequired}})
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, created)
}

func (h *PartnerHandler) get(w http.ResponseWriter, r *http.Request, orgID string) {
	wsID := workspaceID(r)
	p, err := h.store.Get(r.Context(), orgID, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, p)
}

func (h *PartnerHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter := directory.PartnerListFilter{
		PartnerRole: q.Get("partner_role"),
		CertStatus:  q.Get("cert_status"),
	}
	items, next, err := h.store.List(r.Context(), wsID, q.Get("cursor"), queryLimit(r), filter)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func parsePartnerDateField(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
