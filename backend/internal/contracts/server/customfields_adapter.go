package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// CustomFieldsAdapter implements the CustomFields tag's slice of types.ServerInterface.
// CustomFields has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type CustomFieldsAdapter struct{}

// ListCustomFields is unimplemented; see CustomFieldsAdapter's doc comment.
func (CustomFieldsAdapter) ListCustomFields(w http.ResponseWriter, r *http.Request, params types.ListCustomFieldsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateCustomField is unimplemented; see CustomFieldsAdapter's doc comment.
func (CustomFieldsAdapter) CreateCustomField(w http.ResponseWriter, r *http.Request, params types.CreateCustomFieldParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RenameCustomField is unimplemented; see CustomFieldsAdapter's doc comment.
func (CustomFieldsAdapter) RenameCustomField(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RenameCustomFieldParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RetireCustomField is unimplemented; see CustomFieldsAdapter's doc comment.
func (CustomFieldsAdapter) RetireCustomField(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RetireCustomFieldParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
