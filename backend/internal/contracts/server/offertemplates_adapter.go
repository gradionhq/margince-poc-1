//nolint:dupl // adapter boilerplate is structurally identical to ProductsAdapter by design
package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// OfferTemplatesAdapter implements the OfferTemplates tag's slice of types.ServerInterface.
// OfferTemplates has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type OfferTemplatesAdapter struct{}

// ListOfferTemplates is unimplemented; see OfferTemplatesAdapter's doc comment.
func (OfferTemplatesAdapter) ListOfferTemplates(w http.ResponseWriter, r *http.Request, params types.ListOfferTemplatesParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateOfferTemplate is unimplemented; see OfferTemplatesAdapter's doc comment.
func (OfferTemplatesAdapter) CreateOfferTemplate(w http.ResponseWriter, r *http.Request, params types.CreateOfferTemplateParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetOfferTemplate is unimplemented; see OfferTemplatesAdapter's doc comment.
func (OfferTemplatesAdapter) GetOfferTemplate(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateOfferTemplate is unimplemented; see OfferTemplatesAdapter's doc comment.
func (OfferTemplatesAdapter) UpdateOfferTemplate(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateOfferTemplateParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveOfferTemplate is unimplemented; see OfferTemplatesAdapter's doc comment.
func (OfferTemplatesAdapter) ArchiveOfferTemplate(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
