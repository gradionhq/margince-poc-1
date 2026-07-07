package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// TagsAdapter implements the Tags tag's slice of types.ServerInterface.
// Tags has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type TagsAdapter struct{}

// ListTags is unimplemented; see TagsAdapter's doc comment.
func (TagsAdapter) ListTags(w http.ResponseWriter, r *http.Request, params types.ListTagsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateTag is unimplemented; see TagsAdapter's doc comment.
func (TagsAdapter) CreateTag(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveTag is unimplemented; see TagsAdapter's doc comment.
func (TagsAdapter) ArchiveTag(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ApplyTag is unimplemented; see TagsAdapter's doc comment.
func (TagsAdapter) ApplyTag(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
