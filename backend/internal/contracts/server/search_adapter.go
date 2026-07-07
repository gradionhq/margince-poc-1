package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// SearchAdapter implements the Search tag's slice of types.ServerInterface.
// Search has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type SearchAdapter struct{}

// Search is unimplemented; see SearchAdapter's doc comment.
func (SearchAdapter) Search(w http.ResponseWriter, r *http.Request, params types.SearchParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
