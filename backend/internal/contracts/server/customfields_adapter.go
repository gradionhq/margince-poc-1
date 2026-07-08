package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
)

// CustomFieldsAdapter implements the CustomFields tag's slice of
// types.ServerInterface. CreateCustomField delegates to the real governed
// add-field engine's HTTP handler — cmd/api/routes.go wires the identical
// *customfields.Handler instance onto the live mux for POST /custom-fields
// (CF-T03), the same delegation shape OrganizationsAdapter/IdentityAdapter
// use for their wired operations. The other three operations (list, rename,
// retire) stay 501: CF-T02 contracted them, wiring them is a future
// ticket's job (spec Out of scope).
type CustomFieldsAdapter struct {
	H *customfields.Handler
}

// ListCustomFields is unimplemented; see CustomFieldsAdapter's doc comment.
func (a CustomFieldsAdapter) ListCustomFields(w http.ResponseWriter, r *http.Request, params types.ListCustomFieldsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateCustomField delegates to the wired handler; see the struct doc comment above.
func (a CustomFieldsAdapter) CreateCustomField(w http.ResponseWriter, r *http.Request, params types.CreateCustomFieldParams) {
	a.H.ServeHTTP(w, r)
}

// RenameCustomField is unimplemented; see CustomFieldsAdapter's doc comment.
func (a CustomFieldsAdapter) RenameCustomField(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RenameCustomFieldParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RetireCustomField is unimplemented; see CustomFieldsAdapter's doc comment.
func (a CustomFieldsAdapter) RetireCustomField(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RetireCustomFieldParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
