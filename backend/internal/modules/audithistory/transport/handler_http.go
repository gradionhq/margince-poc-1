// Package transport: shared HTTP/JSON helpers used across every handler file
// in this package. Split out of handler_audit_history.go (one concept per file).
package transport

import (
	"encoding/json"
	"net/http"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const (
	fieldData      = "data"
	fieldCode      = "code"
	fieldStatus    = "status"
	codeForbidden  = "forbidden"
	codeBadRequest = "bad_request"
)

func workspaceID(r *http.Request) string {
	p, _ := crmctx.From(r.Context())
	return p.TenantID
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck,gosec // best-effort response body; the status line already conveys success
}

// jsonErr maps a domain error to its HTTP status via the single errs.HTTPStatus
// choke point, then emits a problem+json body.
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
