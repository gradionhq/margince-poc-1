package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	offerstransport "github.com/gradionhq/margince/backend/internal/modules/offers/transport"
)

// OffersAdapter implements the Offers tag's slice of types.ServerInterface.
// ListDealOffers/CreateDealOffer/GetOffer/UpdateOffer/ListOfferLineItems/
// CreateOfferLineItem/UpdateOfferLineItem/DeleteOfferLineItem delegate to the
// real OfferHandler cmd/api/routes.go wires (mirrors ProductsAdapter's
// shape). RegenerateOffer delegates too; RenderOffer/SendOffer/AcceptOffer
// stay 501 stubs — a separate ticket owns the sent->accepted transitions and
// ApprovalToken gating.
type OffersAdapter struct {
	H *offerstransport.OfferHandler
}

// ListDealOffers delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) ListDealOffers(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.ListDealOffersParams) {
	a.H.ServeHTTP(w, r)
}

// CreateDealOffer delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) CreateDealOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.CreateDealOfferParams) {
	a.H.ServeHTTP(w, r)
}

// GetOffer delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) GetOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdateOffer delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) UpdateOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateOfferParams) {
	a.H.ServeHTTP(w, r)
}

// ListOfferLineItems delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) ListOfferLineItems(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// CreateOfferLineItem delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) CreateOfferLineItem(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.CreateOfferLineItemParams) {
	a.H.ServeHTTP(w, r)
}

// UpdateOfferLineItem delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) UpdateOfferLineItem(w http.ResponseWriter, r *http.Request, idParam types.IdParam, lineID openapi_types.UUID, params types.UpdateOfferLineItemParams) {
	a.H.ServeHTTP(w, r)
}

// DeleteOfferLineItem delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) DeleteOfferLineItem(w http.ResponseWriter, r *http.Request, idParam types.IdParam, lineID openapi_types.UUID) {
	a.H.ServeHTTP(w, r)
}

// RegenerateOffer delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) RegenerateOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RegenerateOfferParams) {
	a.H.ServeHTTP(w, r)
}

// RenderOffer is unimplemented; see the struct doc comment above — a separate ticket.
func (a *OffersAdapter) RenderOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RenderOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// SendOffer is unimplemented; see the struct doc comment above — a separate ticket.
func (a *OffersAdapter) SendOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.SendOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// AcceptOffer delegates to the wired handler; see the struct doc comment above.
func (a *OffersAdapter) AcceptOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.AcceptOfferParams) {
	a.H.ServeHTTP(w, r)
}
