// Package transport holds the partners module's HTTP handler helpers.
// Shared HTTP/JSON helpers used across every handler file in this package.
// The small set of helpers below are deliberately duplicated from
// modules/directory/transport rather than exported solely for this package's
// benefit — same minimal-duplication class as other transport packages.
package transport

import (
	"encoding/json"
	"net/http"
	"strconv"

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

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec
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
