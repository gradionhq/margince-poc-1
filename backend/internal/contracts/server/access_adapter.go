package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// AccessAdapter implements the Access tag's slice of types.ServerInterface.
// Access has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type AccessAdapter struct{}

// ListRecordGrants is unimplemented; see AccessAdapter's doc comment.
func (AccessAdapter) ListRecordGrants(w http.ResponseWriter, r *http.Request, params types.ListRecordGrantsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateRecordGrant is unimplemented; see AccessAdapter's doc comment.
func (AccessAdapter) CreateRecordGrant(w http.ResponseWriter, r *http.Request, params types.CreateRecordGrantParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RevokeRecordGrant is unimplemented; see AccessAdapter's doc comment.
func (AccessAdapter) RevokeRecordGrant(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RevokeRecordGrantParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
