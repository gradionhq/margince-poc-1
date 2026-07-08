package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// OffersAdapter implements the Offers tag's slice of types.ServerInterface.
// Offers has no wired handler in this pruned skeleton tree (AC-D3/D10 scope: interface
// conformance, not a live-routing cutover) — every method returns 501 via the same
// shape oapi-codegen's own generated types.Unimplemented stub uses.
type OffersAdapter struct{}

// ListDealOffers is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) ListDealOffers(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.ListDealOffersParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateDealOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) CreateDealOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.CreateDealOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) GetOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) UpdateOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// ListOfferLineItems is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) ListOfferLineItems(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CreateOfferLineItem is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) CreateOfferLineItem(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.CreateOfferLineItemParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateOfferLineItem is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) UpdateOfferLineItem(w http.ResponseWriter, r *http.Request, idParam types.IdParam, lineID openapi_types.UUID, params types.UpdateOfferLineItemParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// DeleteOfferLineItem is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) DeleteOfferLineItem(w http.ResponseWriter, r *http.Request, idParam types.IdParam, lineID openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RegenerateOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) RegenerateOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RegenerateOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// RenderOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) RenderOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RenderOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// SendOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) SendOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.SendOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// AcceptOffer is unimplemented; see OffersAdapter's doc comment.
func (OffersAdapter) AcceptOffer(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.AcceptOfferParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
