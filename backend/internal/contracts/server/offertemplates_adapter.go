package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	offerstransport "github.com/gradionhq/margince/backend/internal/modules/offers/transport"
)

// OfferTemplatesAdapter implements the OfferTemplates tag's slice of
// types.ServerInterface by delegating to the real OfferTemplateHandler
// cmd/api/routes.go wires for /offer-templates.
type OfferTemplatesAdapter struct {
	H *offerstransport.OfferTemplateHandler
}

// ListOfferTemplates delegates to the wired handler; see the struct doc comment above.
func (a *OfferTemplatesAdapter) ListOfferTemplates(w http.ResponseWriter, r *http.Request, params types.ListOfferTemplatesParams) {
	a.H.ServeHTTP(w, r)
}

// CreateOfferTemplate delegates to the wired handler; see the struct doc comment above.
func (a *OfferTemplatesAdapter) CreateOfferTemplate(w http.ResponseWriter, r *http.Request, params types.CreateOfferTemplateParams) {
	a.H.ServeHTTP(w, r)
}

// GetOfferTemplate delegates to the wired handler; see the struct doc comment above.
func (a *OfferTemplatesAdapter) GetOfferTemplate(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdateOfferTemplate delegates to the wired handler; see the struct doc comment above.
func (a *OfferTemplatesAdapter) UpdateOfferTemplate(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateOfferTemplateParams) {
	a.H.ServeHTTP(w, r)
}

// ArchiveOfferTemplate delegates to the wired handler; see the struct doc comment above.
func (a *OfferTemplatesAdapter) ArchiveOfferTemplate(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
