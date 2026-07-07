package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// AutomationsAdapter implements the Automations tag's slice of types.ServerInterface.
// Automations has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type AutomationsAdapter struct{}

// ListAutomations is unimplemented; see AutomationsAdapter's doc comment.
func (AutomationsAdapter) ListAutomations(w http.ResponseWriter, r *http.Request, params types.ListAutomationsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateAutomation is unimplemented; see AutomationsAdapter's doc comment.
func (AutomationsAdapter) CreateAutomation(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetAutomation is unimplemented; see AutomationsAdapter's doc comment.
func (AutomationsAdapter) GetAutomation(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// PatchAutomation is unimplemented; see AutomationsAdapter's doc comment.
func (AutomationsAdapter) PatchAutomation(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// DeleteAutomation is unimplemented; see AutomationsAdapter's doc comment.
func (AutomationsAdapter) DeleteAutomation(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
