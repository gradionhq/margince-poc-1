package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	customfields "github.com/gradionhq/margince/backend/internal/platform/customfields"
)

// CustomFieldsAdapter implements the CustomFields tag's slice of
// types.ServerInterface. CreateCustomField/RenameCustomField/RetireCustomField/
// UpdateCustomFieldOptions all delegate to the same real governed engine's
// HTTP handler — cmd/api/routes.go wires the identical *customfields.Handler
// instance onto the live mux for /custom-fields (CF-T03/CF-T04), the same
// delegation shape ActivitiesAdapter uses for its wired operations: the
// wrapped Handler re-parses the id (and any /retire, /options suffix) from
// r.URL.Path itself, so this adapter needs no id-plumbing. ListCustomFields
// stays 501 — CF-T02 contracted it, wiring it is a separate future ticket
// (spec Out of scope).
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

// RenameCustomField delegates to the wired handler; see the struct doc comment above.
func (a CustomFieldsAdapter) RenameCustomField(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RenameCustomFieldParams) {
	a.H.ServeHTTP(w, r)
}

// RetireCustomField delegates to the wired handler; see the struct doc comment above.
func (a CustomFieldsAdapter) RetireCustomField(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RetireCustomFieldParams) {
	a.H.ServeHTTP(w, r)
}

// UpdateCustomFieldOptions delegates to the wired handler; see the struct doc comment above.
func (a CustomFieldsAdapter) UpdateCustomFieldOptions(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateCustomFieldOptionsParams) {
	a.H.ServeHTTP(w, r)
}
