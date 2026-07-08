package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ExportsAdapter implements the Exports tag's slice of types.ServerInterface.
// Exports has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ExportsAdapter struct{}

// CreateExport is unimplemented; see ExportsAdapter's doc comment.
func (ExportsAdapter) CreateExport(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetExport is unimplemented; see ExportsAdapter's doc comment.
func (ExportsAdapter) GetExport(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}
