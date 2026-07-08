// Package transport contains the HTTP handlers for the audithistory module.
package transport

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/ports"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/authz"
)

// HistoryHandler serves GET /records/{entity_type}/{id}/history.
type HistoryHandler struct {
	reader ports.HistoryReader
	authz  authz.Authorizer
}

// NewHistoryHandler returns a HistoryHandler.
func NewHistoryHandler(reader ports.HistoryReader, az authz.Authorizer) *HistoryHandler {
	return &HistoryHandler{reader: reader, authz: az}
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

	if !authz.ReUUID.MatchString(id) {
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
		entries = []domain.AuditHistoryEntry{}
	}
	jsonOK(w, map[string]any{fieldData: entries})
}
