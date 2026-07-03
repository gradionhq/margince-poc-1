// Package transport holds the deal module's HTTP handler for /deals
// (createDeal, updateDeal, listDeals — T11). Mirrors
// modules/people/transport's package layout and its documented
// minimal-duplication convention: small HTTP helpers below are deliberately
// duplicated from people/transport rather than exported solely for this
// handler's benefit.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const (
	fieldData      = "data"
	fieldCode      = "code"
	fieldStatus    = "status"
	fieldDetails   = "details"
	codeBadRequest = "bad_request"
	codeValidation = "validation_error"
)

// DealHandler routes /deals and /deals/{id} requests to the DealStore.
type DealHandler struct{ store *directory.DealStore }

// NewDealHandler returns a DealHandler.
func NewDealHandler(store *directory.DealStore) *DealHandler {
	return &DealHandler{store: store}
}

// ServeHTTP dispatches on method + path suffix. Only POST /deals is
// implemented by this task; PATCH /deals/{id} (Task 2) and GET /deals
// (Task 3) add their cases below as they land.
func (h *DealHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/deals")
	switch {
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *DealHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	var body struct {
		Name              string  `json:"name"`
		AmountMinor       *int64  `json:"amount_minor"`
		Currency          *string `json:"currency"`
		PipelineID        string  `json:"pipeline_id"`
		StageID           string  `json:"stage_id"`
		OrganizationID    *string `json:"organization_id"`
		OwnerID           *string `json:"owner_id"`
		ExpectedCloseDate *string `json:"expected_close_date"`
		Source            string  `json:"source"`
		CapturedBy        string  `json:"captured_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.Name == "" || body.PipelineID == "" || body.StageID == "" || body.Source == "" || body.CapturedBy == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_required_fields")
		return
	}

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" {
		if existing, ok, err := h.store.FindByIdempotencyKey(r.Context(), wsID, idemKey); err != nil {
			jsonErr(w, err)
			return
		} else if ok {
			jsonCreatedAt(w, existing, "/deals/"+existing.ID)
			return
		}
	}

	d := directory.NewDeal(body.Name, body.PipelineID, body.StageID,
		provenanceOf(body.Source, body.CapturedBy))
	d.WorkspaceID = wsID
	d.AmountMinor = body.AmountMinor
	d.Currency = body.Currency
	d.OrganizationID = body.OrganizationID
	d.OwnerID = body.OwnerID
	if body.ExpectedCloseDate != nil {
		if t, err := time.Parse("2006-01-02", *body.ExpectedCloseDate); err == nil {
			d.ExpectedCloseDate = &t
		} else {
			jsonProblem(w, http.StatusBadRequest, "bad_expected_close_date")
			return
		}
	}

	created, err := h.store.Create(r.Context(), d, idemKey)
	if errors.Is(err, errs.ErrStageNotInPipeline) {
		jsonValidationError(w, "stage_id does not belong to pipeline_id.",
			[]fieldError{{Field: "stage_id", Code: "stage_not_in_pipeline"}})
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonCreatedAt(w, created, "/deals/"+created.ID)
}

// ---------------------------------------------------------------------------
// shared HTTP helpers (unexported, used across handler files in this package)
// ---------------------------------------------------------------------------

func pathID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func workspaceID(r *http.Request) string {
	p, _ := crmctx.From(r.Context())
	return p.TenantID
}

func queryLimit(r *http.Request, def int) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// parseIfMatch mirrors people/transport's helper of the same name exactly
// (see that file's doc comment for the three-way contract).
func parseIfMatch(r *http.Request) (version int64, malformed bool) {
	s := r.Header.Get("If-Match")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, true
	}
	return v, false
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

// jsonCreatedAt writes 201 with a Location header pointing at the created
// resource, per createDeal's contract response.
func jsonCreatedAt(w http.ResponseWriter, v any, location string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

type fieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// jsonValidationError writes a 422 problem+json body with the field-level
// details.errors shape the contract's ValidationError schema declares.
func jsonValidationError(w http.ResponseWriter, detail string, errs []fieldError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		fieldStatus:  http.StatusUnprocessableEntity,
		fieldCode:    codeValidation,
		"detail":     detail,
		fieldDetails: map[string]any{"errors": errs},
	})
}

func jsonErr(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	jsonProblem(w, status, problemCode(status))
}

func problemCode(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusTooManyRequests:
		return "budget_exceeded"
	case http.StatusUnprocessableEntity:
		return codeValidation
	case http.StatusUnavailableForLegalReasons:
		return "suppressed"
	case http.StatusBadRequest:
		return codeBadRequest
	default:
		return "internal_error"
	}
}

func jsonProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{fieldStatus: status, fieldCode: code}) //nolint:errcheck,gosec
}

func pageResponse(data any, nextCursor string) map[string]any {
	var next any
	hasMore := nextCursor != ""
	if hasMore {
		next = nextCursor
	}
	return map[string]any{
		fieldData: data,
		"page": map[string]any{
			"next_cursor": next,
			"has_more":    hasMore,
		},
	}
}

func provenanceOf(source, capturedBy string) prov.Provenance {
	return prov.Provenance{Source: source, CapturedBy: capturedBy}
}
