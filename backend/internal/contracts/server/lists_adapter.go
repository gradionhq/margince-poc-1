package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// ListsAdapter implements the Lists tag's slice of types.ServerInterface.
// Lists has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ListsAdapter struct{}

// ListLists is unimplemented; see ListsAdapter's doc comment.
func (ListsAdapter) ListLists(w http.ResponseWriter, r *http.Request, params types.ListListsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateList is unimplemented; see ListsAdapter's doc comment.
func (ListsAdapter) CreateList(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetList is unimplemented; see ListsAdapter's doc comment.
func (ListsAdapter) GetList(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveList is unimplemented; see ListsAdapter's doc comment.
func (ListsAdapter) ArchiveList(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ListListMembers is unimplemented; see ListsAdapter's doc comment.
func (ListsAdapter) ListListMembers(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.ListListMembersParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// AddListMember is unimplemented; see ListsAdapter's doc comment.
func (ListsAdapter) AddListMember(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
