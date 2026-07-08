package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
)

type quotaStoreSeam interface {
	Create(ctx context.Context, q records.Quota) (records.Quota, error)
	Get(ctx context.Context, id, workspaceID string) (records.Quota, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool, filter records.QuotaListFilter) ([]records.Quota, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (records.Quota, error)
	Archive(ctx context.Context, id, workspaceID string) (records.Quota, error)
	Attainment(ctx context.Context, id, workspaceID string) (records.Attainment, error)
}

// QuotaHandler routes /quotas and /quotas/{id} requests to the quota store.
type QuotaHandler struct{ store quotaStoreSeam }

// NewQuotaHandler returns a QuotaHandler backed by the given store.
func NewQuotaHandler(store quotaStoreSeam) *QuotaHandler { return &QuotaHandler{store: store} }

func (h *QuotaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.serveSuffixRoutes(w, r) {
		return
	}
	id := httpkit.PathID(r.URL.Path, "/quotas")
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

// serveSuffixRoutes dispatches /attainment sub-resource reads, keeping ServeHTTP
// cyclomatic complexity within budget (mirrors organizations/transport's pattern).
func (h *QuotaHandler) serveSuffixRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/attainment") {
		id := httpkit.PathID(strings.TrimSuffix(r.URL.Path, "/attainment"), "/quotas")
		h.attainment(w, r, id)
		return true
	}
	return false
}

func (h *QuotaHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := httpkit.WorkspaceID(r)
	var body struct {
		OwnerID     *string `json:"owner_id"`
		TeamID      *string `json:"team_id"`
		PeriodStart string  `json:"period_start"`
		PeriodEnd   string  `json:"period_end"`
		TargetMinor int64   `json:"target_minor"`
		Currency    string  `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	periodStart, err := time.Parse("2006-01-02", body.PeriodStart)
	if err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	periodEnd, err := time.Parse("2006-01-02", body.PeriodEnd)
	if err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	q := records.Quota{
		WorkspaceID: wsID,
		OwnerID:     body.OwnerID,
		TeamID:      body.TeamID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TargetMinor: body.TargetMinor,
		Currency:    body.Currency,
	}
	created, err := h.store.Create(r.Context(), q)
	if errors.Is(err, records.ErrOwnerXorTeamRequired) {
		httpkit.JSONValidationError(w, "exactly one of owner_id and team_id is required.",
			[]fieldError{{Field: "owner_id", Code: "owner_xor_team_required"}})
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONCreatedAt(w, created, "/quotas/"+created.ID)
}

func (h *QuotaHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	q, err := h.store.Get(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, q)
}

func (h *QuotaHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter := records.QuotaListFilter{
		OwnerID: q.Get("owner_id"),
		TeamID:  q.Get("team_id"),
	}
	includeArchived := q.Get("include_archived") == "true"
	items, next, err := h.store.List(r.Context(), wsID, q.Get("cursor"), httpkit.QueryLimit(r, 20), includeArchived, filter)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, httpkit.PageResponse(items, next))
}

func (h *QuotaHandler) update(w http.ResponseWriter, r *http.Request, id string) {
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
	// Validate date fields if present — store reads them as raw strings via sqlutil.NullStr,
	// so we pre-validate here and leave them as strings in the map.
	for _, dateField := range []string{"period_start", "period_end"} {
		if v, ok := body[dateField]; ok && v != nil {
			s, isStr := v.(string)
			if !isStr {
				httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
				return
			}
			if _, err := time.Parse("2006-01-02", s); err != nil {
				httpkit.JSONProblem(w, http.StatusBadRequest, codeBadRequest)
				return
			}
		}
	}
	updated, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	if errors.Is(err, records.ErrOwnerXorTeamRequired) {
		httpkit.JSONValidationError(w, "exactly one of owner_id and team_id is required.",
			[]fieldError{{Field: "owner_id", Code: "owner_xor_team_required"}})
		return
	}
	if errors.Is(err, errs.ErrVersionSkew) {
		httpkit.JSONProblem(w, http.StatusConflict, "version_skew")
		return
	}
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, updated)
}

func (h *QuotaHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	archived, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, archived)
}
