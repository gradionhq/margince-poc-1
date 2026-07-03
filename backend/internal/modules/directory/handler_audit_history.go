package crmcore

import (
	"encoding/json"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// HistoryHandler serves GET /records/{entity_type}/{id}/history.
// It uses the Authorizer func type already declared in authz.go (same
// package), so no re-declaration is needed here.
type HistoryHandler struct {
	reader *AuditHistoryReader
	authz  Authorizer
}

// NewHistoryHandler returns a HistoryHandler.
func NewHistoryHandler(reader *AuditHistoryReader, authz Authorizer) *HistoryHandler {
	return &HistoryHandler{reader: reader, authz: authz}
}

// ServeHTTP handles GET /records/{entity_type}/{id}/history.
// entity_type and id are Go 1.22 wildcard path values set by the mux.
func (h *HistoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entityType := r.PathValue("entity_type")
	id := r.PathValue("id")

	if entityType == "" || id == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_path_params")
		return
	}

	if !reUUID.MatchString(id) {
		jsonProblem(w, http.StatusBadRequest, "invalid_id")
		return
	}

	// Object-level RBAC gate: the viewer must have `read` on the entity_type.
	if err := h.authz(r.Context(), entityType, "read"); err != nil {
		jsonProblem(w, http.StatusForbidden, codeForbidden)
		return
	}

	wsID := workspaceID(r)
	if wsID == "" {
		jsonProblem(w, http.StatusBadRequest, "missing_workspace")
		return
	}

	entries, err := h.reader.ReadHistory(r.Context(), entityType, id, wsID)
	if err != nil {
		jsonErr(w, err)
		return
	}

	if entries == nil {
		entries = []AuditHistoryEntry{}
	}
	jsonOK(w, map[string]any{fieldData: entries})
}

// ---------------------------------------------------------------------------
// HTTP response helpers, local to this file.
//
// These duplicate modules/people/transport/handler_person.go's helpers of the
// same name (jsonOK, jsonErr, jsonProblem, workspaceID, problemCode): both
// files shared one package-private copy in crm-core before the 1c restructure
// split it into modules/people (handler_person.go, per mapping) and
// modules/directory (handler_audit_history.go, the generic/unmounted
// record-history endpoint — see task-3-report.md's mapping-deviations
// section). Exporting them solely to keep one shared copy across the new
// package boundary is not authorized by this task; duplicating this small,
// generic HTTP/JSON plumbing is the minimal mechanical fix, same class as
// httpserver's keyStatus (see internal/platform/httpserver/middleware.go).
// ---------------------------------------------------------------------------

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
