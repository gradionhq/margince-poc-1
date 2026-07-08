//nolint:dupl // adapter boilerplate is structurally identical to OfferTemplatesAdapter by design
package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// ProductsAdapter implements the Products tag's slice of types.ServerInterface.
// Products has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type ProductsAdapter struct{}

// ListProducts is unimplemented; see ProductsAdapter's doc comment.
func (ProductsAdapter) ListProducts(w http.ResponseWriter, r *http.Request, params types.ListProductsParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateProduct is unimplemented; see ProductsAdapter's doc comment.
func (ProductsAdapter) CreateProduct(w http.ResponseWriter, r *http.Request, params types.CreateProductParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetProduct is unimplemented; see ProductsAdapter's doc comment.
func (ProductsAdapter) GetProduct(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateProduct is unimplemented; see ProductsAdapter's doc comment.
func (ProductsAdapter) UpdateProduct(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateProductParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ArchiveProduct is unimplemented; see ProductsAdapter's doc comment.
func (ProductsAdapter) ArchiveProduct(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}
