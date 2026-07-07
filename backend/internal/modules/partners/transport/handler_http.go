// Package transport holds the partners module's HTTP handler helpers.
// Shared HTTP/JSON helpers used across every handler file in this package.
// The small set of helpers below are deliberately duplicated from
// modules/directory/transport rather than exported solely for this package's
// benefit — same minimal-duplication class as other transport packages.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const (
	fieldData       = "data"
	fieldCode       = "code"
	fieldStatus     = "status"
	fieldDetails    = "details"
	codeBadRequest  = "bad_request"
	codeValidation  = "validation_error"
	codeRequired    = "required"
	fieldCapturedBy = "captured_by"
	fieldSource     = "source"
)

type fieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

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

func requireWorkspace(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := workspaceID(r)
	if id == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_workspace")
		return "", false
	}
	return id, true
}

func queryLimit(r *http.Request) int {
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 20
}

// parseIfMatch returns the version from the If-Match header, or 0 if absent,
// or malformed=true when the value is present but not a valid int64.
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

// parseQuery returns the request's URL query parameters.
func parseQuery(r *http.Request) url.Values {
	return r.URL.Query()
}

// paginationParams extracts cursor and limit from the request query string.
func paginationParams(r *http.Request) (cursor string, limit int) {
	return r.URL.Query().Get("cursor"), queryLimit(r)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
}

// textResponse writes a plain-text response with the given status code.
func textResponse(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(text))
}

// setCursor sets the X-Next-Cursor response header when cursor is non-empty.
func setCursor(w http.ResponseWriter, cursor string) {
	if cursor != "" {
		w.Header().Set("X-Next-Cursor", cursor)
	}
}

// jsonValidationError writes a 422 problem+json body with field-level details.
func jsonValidationError(w http.ResponseWriter, detail string, fieldErrs []fieldError) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,gosec
		fieldStatus:  http.StatusUnprocessableEntity,
		fieldCode:    codeValidation,
		"detail":     detail,
		fieldDetails: map[string]any{"errors": fieldErrs},
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

// writeUpdateResult maps a store Update/Patch error to its problem+json
// response, or writes v as 200 on success.
func writeUpdateResult[T any](w http.ResponseWriter, v T, err error) {
	if errors.Is(err, errs.ErrConflict) {
		jsonProblem(w, http.StatusConflict, "conflict")
		return
	}
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
	jsonOK(w, v)
}
