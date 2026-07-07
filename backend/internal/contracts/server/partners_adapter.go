package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	dealtransport "github.com/gradionhq/margince/backend/internal/modules/directory/transport"
)

// PartnersAdapter implements the Partners tag's slice of
// types.ServerInterface by delegating to the real PartnerHandler
// cmd/api/routes.go already wires for /partners and /organizations/{id}/partner.
type PartnersAdapter struct {
	H *dealtransport.PartnerHandler
}

// UpsertPartner delegates to the wired handler; see the struct doc comment above.
func (a *PartnersAdapter) UpsertPartner(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// GetPartner delegates to the wired handler; see the struct doc comment above.
func (a *PartnersAdapter) GetPartner(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// ListPartners delegates to the wired handler; see the struct doc comment above.
func (a *PartnersAdapter) ListPartners(w http.ResponseWriter, r *http.Request, params types.ListPartnersParams) {
	a.H.ServeHTTP(w, r)
}
