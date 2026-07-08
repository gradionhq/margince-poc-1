// Package httpkit is the Tier-0 kernel of generic HTTP/JSON transport helpers
// shared by the per-entity modules' transport packages: workspace/path
// extraction, optimistic-concurrency header parsing, and the problem+json
// response shapes (RFC 7807) every module renders identically.
//
// These were copy-pasted verbatim into every catalog module's transport package
// during the directory-split (WS-E-b); consolidating them here keeps one
// canonical rendering of the API's error/pagination envelope. Anything
// entity-specific (route dispatch, request DTOs, provenance mapping) stays in
// each module's transport package.
package httpkit

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// JSON envelope keys and machine-readable codes shared by every module's
// problem+json and list responses.
const (
	fieldData      = "data"
	fieldCode      = "code"
	fieldStatus    = "status"
	fieldDetails   = "details"
	codeBadRequest = "bad_request"
	codeValidation = "validation_error"
)

// PathID extracts the first path segment after prefix (e.g. the resource id in
// /organizations/{id}/... ), ignoring any trailing sub-route.
func PathID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimPrefix(rest, "/")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// WorkspaceID returns the tenant id of the authenticated principal, or "".
func WorkspaceID(r *http.Request) string {
	p, _ := crmctx.From(r.Context())
	return p.TenantID
}

// RequireWorkspace resolves the workspace id or writes a 400 missing_workspace
// problem and reports ok=false.
func RequireWorkspace(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := WorkspaceID(r)
	if id == "" {
		JSONProblem(w, http.StatusBadRequest, "missing_workspace")
		return "", false
	}
	return id, true
}

// QueryLimit reads the ?limit= query parameter, falling back to def when it is
// absent or unparseable.
func QueryLimit(r *http.Request, def int) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// ParseIfMatch parses the optional If-Match header into an optimistic-concurrency
// version. A malformed header is reported distinctly so it is never silently
// downgraded to last-write-wins.
func ParseIfMatch(r *http.Request) (version int64, malformed bool) {
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

// JSONOK writes v as a 200 application/json body.
func JSONOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

// JSONCreatedAt writes 201 with a Location header pointing at the created resource.
func JSONCreatedAt(w http.ResponseWriter, v any, location string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

// FieldError is one field-level entry in a validation-error response.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// JSONValidationError writes a 422 problem+json body with field-level details.
func JSONValidationError(w http.ResponseWriter, detail string, ferrs []FieldError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		fieldStatus:  http.StatusUnprocessableEntity,
		fieldCode:    codeValidation,
		"detail":     detail,
		fieldDetails: map[string]any{"errors": ferrs},
	})
}

// JSONError maps err to its HTTP status and renders the matching problem+json body.
func JSONError(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	JSONProblem(w, status, ProblemCode(status))
}

// ProblemCode maps an HTTP status to its machine-readable problem code.
func ProblemCode(status int) string {
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

// JSONProblem writes a minimal status+code problem+json body.
func JSONProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{fieldStatus: status, fieldCode: code}) //nolint:errcheck,gosec
}

// JSONProblemDetails writes a problem+json body with an arbitrary details map,
// for errors whose machine-readable code needs request-specific data beyond the
// plain status+code JSONProblem covers.
func JSONProblemDetails(w http.ResponseWriter, status int, code, detail string, details map[string]any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		fieldStatus:  status,
		fieldCode:    code,
		"detail":     detail,
		fieldDetails: details,
	})
}

// WriteUpdateResult maps a store Update/Patch error to its problem+json response
// (conflict / version_skew / not_found / the generic JSONError fallback), or
// writes v as a 200 on success.
func WriteUpdateResult[T any](w http.ResponseWriter, v T, err error) {
	if errors.Is(err, errs.ErrConflict) {
		JSONProblem(w, http.StatusConflict, "conflict")
		return
	}
	if errors.Is(err, errs.ErrVersionSkew) {
		JSONProblem(w, http.StatusConflict, "version_skew")
		return
	}
	if errors.Is(err, errs.ErrNotFound) {
		JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		JSONError(w, err)
		return
	}
	JSONOK(w, v)
}

// PageResponse wraps a data slice with the cursor-pagination envelope.
func PageResponse(data any, nextCursor string) map[string]any {
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
