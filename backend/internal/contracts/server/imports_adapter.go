package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ImportsAdapter implements the Imports tag's slice of types.ServerInterface.
// Imports has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ImportsAdapter struct{}

// CreateImport is unimplemented; see ImportsAdapter's doc comment.
func (ImportsAdapter) CreateImport(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetImport is unimplemented; see ImportsAdapter's doc comment.
func (ImportsAdapter) GetImport(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ApproveImport is unimplemented; see ImportsAdapter's doc comment.
func (ImportsAdapter) ApproveImport(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}
