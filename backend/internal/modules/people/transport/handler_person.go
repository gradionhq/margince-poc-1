// Package transport holds the people module's HTTP handler for /people
// (extracted from crm-core's handler_person.go, 1c restructure,
// task-3-brief.md). Person's domain type and store stayed in
// modules/directory rather than splitting into modules/people/{domain,adapters}
// (see task-3-report.md's mapping-deviations section: Person is fused with the
// Lead-promotion spine transaction and the DatasourceProvider seam via
// unexported same-package access that a split would have forced to export —
// not authorized by this task). This handler references directory's exported
// Person/PersonStore/NewPerson/NewPersonStore API only; the small set of HTTP
// helpers and problem-code constants below were previously shared,
// package-private, with handler_audit_history.go (which stayed in directory)
// and are duplicated here rather than exported solely for this handler's
// benefit — same minimal-duplication class as httpserver's keyStatus (see
// internal/platform/httpserver/middleware.go).
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// problem+json response field names and machine-readable codes, and the
// contract-envelope field name, this handler needs. Duplicated from
// directory's consts.go (see package doc above).
const (
	fieldData      = "data"
	fieldCode      = "code"
	fieldStatus    = "status"
	codeForbidden  = "forbidden"
	codeBadRequest = "bad_request"
)

var personSortValues = map[string]bool{
	"": true, "id": true, "strength": true, "-strength": true,
}

// PersonHandler routes /people and /people/{id} requests to the PersonStore.
type PersonHandler struct{ store *directory.PersonStore }

// NewPersonHandler returns a PersonHandler.
func NewPersonHandler(store *directory.PersonStore) *PersonHandler {
	return &PersonHandler{store: store}
}

// ServeHTTP dispatches on method + path suffix.
func (h *PersonHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := pathID(r.URL.Path, "/people")
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

func (h *PersonHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := requireWorkspace(w, r)
	if !ok {
		return
	}
	sort := r.URL.Query().Get("sort")
	if !personSortValues[sort] {
		jsonProblem(w, http.StatusUnprocessableEntity, "sort_field_not_allowed")
		return
	}
	cursor := r.URL.Query().Get("cursor")
	limit := queryLimit(r, 20)
	items, next, err := h.store.List(r.Context(), wsID, cursor, limit, sort)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, pageResponse(items, next))
}

func (h *PersonHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID := workspaceID(r)
	var body struct {
		FullName   string  `json:"full_name"`
		FirstName  *string `json:"first_name"`
		LastName   *string `json:"last_name"`
		Title      *string `json:"title"`
		OwnerID    *string `json:"owner_id"`
		Source     string  `json:"source"`
		CapturedBy string  `json:"captured_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonProblem(w, http.StatusBadRequest, codeBadRequest)
		return
	}
	if body.FullName == "" || body.Source == "" || body.CapturedBy == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_required_fields")
		return
	}
	p := directory.NewPerson(body.FullName, prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy})
	p.WorkspaceID = wsID
	p.FirstName = body.FirstName
	p.LastName = body.LastName
	p.Title = body.Title
	p.OwnerID = body.OwnerID
	created, err := h.store.Create(r.Context(), p)
	if err != nil {
		jsonErr(w, err)
		return
	}
	// Audit is written by PersonStore.Create at the store layer (covers direct
	// store callers too), so no handler-level audit write is needed here.
	jsonCreated(w, created)
}

func (h *PersonHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	p, err := h.store.Get(r.Context(), id, wsID)
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

func (h *PersonHandler) update(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	// If-Match is optional: omitting is last-write-wins (version=0 bypasses check).
	// A present-but-malformed If-Match is a client error (400), never a silent skip.
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
	p, err := h.store.Update(r.Context(), id, wsID, body, ifMatch)
	if errors.Is(err, errs.ErrVersionSkew) {
		jsonProblem(w, http.StatusConflict, "version_skew")
		return
	}
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	// Audit written by PersonStore.Update (store layer).
	jsonOK(w, p)
}

func (h *PersonHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := workspaceID(r)
	archived, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		jsonProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		jsonErr(w, err)
		return
	}
	// Audit written by PersonStore.Archive (store layer).
	jsonOK(w, archived)
}

// ---------------------------------------------------------------------------
// shared HTTP helpers (unexported, used across handler files in this package)
// ---------------------------------------------------------------------------

// pathID extracts the trailing path segment after prefix.
// "/people/abc123" with prefix "/people" → "abc123"
func pathID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	// strip any further segments
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func workspaceID(r *http.Request) string {
	p, _ := crmctx.From(r.Context())
	return p.TenantID
}

// requireWorkspace writes a 400 and returns false when the request carries no
// workspace ID (i.e. before WP6 auth lands and the frontend sends the header).
func requireWorkspace(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := workspaceID(r)
	if id == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_workspace")
		return "", false
	}
	return id, true
}

// queryLimit reads the ?limit query parameter as an int, falling back to def when
// the parameter is absent or non-numeric.
func queryLimit(r *http.Request, def int) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// parseIfMatch parses the optional If-Match header into an optimistic-concurrency
// version. A malformed header is reported distinctly so it is never silently
// downgraded to last-write-wins:
//   - absent      -> (0, malformed=false): last-write-wins is intended
//   - present, ok -> (version, false)
//   - present, bad -> (0, malformed=true): caller MUST reject with 400, not skip the check
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
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec // best-effort response body; the status line already conveys success
}

func jsonCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec // best-effort response body; the status line already conveys success
}

// jsonErr maps a domain error to its HTTP status via the single errs.HTTPStatus
// choke point, then emits a problem+json body. Previously it discarded err and always
// emitted 500, swallowing sentinel→status (e.g. a 422 null-provenance read as 500).
func jsonErr(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	jsonProblem(w, status, problemCode(status))
}

// problemCode is the stable machine-readable code string for an HTTP status used in
// problem+json bodies, matching the codes the handlers emit at their explicit sites.
func problemCode(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusForbidden:
		return codeForbidden
	case http.StatusTooManyRequests:
		return "budget_exceeded"
	case http.StatusUnprocessableEntity:
		return "unprocessable"
	case http.StatusUnavailableForLegalReasons:
		return "suppressed"
	case http.StatusBadRequest:
		return codeBadRequest
	default:
		return "internal_error"
	}
}

// jsonProblem writes an RFC 7807 application/problem+json response.
func jsonProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{fieldStatus: status, fieldCode: code}) //nolint:errcheck,gosec // best-effort problem body; the status line already conveys the failure
}

// pageResponse wraps a data slice and cursor into the contract's pagination envelope.
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
