package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	offerstransport "github.com/gradionhq/margince/backend/internal/modules/offers/transport"
)

// ProductsAdapter implements the Products tag's slice of types.ServerInterface
// by delegating to the real ProductHandler cmd/api/routes.go wires for
// /products.
type ProductsAdapter struct {
	H *offerstransport.ProductHandler
}

// ListProducts delegates to the wired handler; see the struct doc comment above.
func (a *ProductsAdapter) ListProducts(w http.ResponseWriter, r *http.Request, params types.ListProductsParams) {
	a.H.ServeHTTP(w, r)
}

// CreateProduct delegates to the wired handler; see the struct doc comment above.
func (a *ProductsAdapter) CreateProduct(w http.ResponseWriter, r *http.Request, params types.CreateProductParams) {
	a.H.ServeHTTP(w, r)
}

// GetProduct delegates to the wired handler; see the struct doc comment above.
func (a *ProductsAdapter) GetProduct(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdateProduct delegates to the wired handler; see the struct doc comment above.
func (a *ProductsAdapter) UpdateProduct(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateProductParams) {
	a.H.ServeHTTP(w, r)
}

// ArchiveProduct delegates to the wired handler; see the struct doc comment above.
func (a *ProductsAdapter) ArchiveProduct(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
