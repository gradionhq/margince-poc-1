package server

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	dealtransport "github.com/gradionhq/margince/backend/internal/modules/directory/transport"
)

// DealsAdapter implements the Deals tag's slice of types.ServerInterface.
// The CRUD + advance/restore/stakeholders surface delegates, unchanged, to
// the real DealHandler cmd/api/routes.go already wires for /deals (its
// ServeHTTP dispatches by request method+path itself). The KPI-signal,
// next-step, and people-signal operations have no handler in this pruned
// skeleton tree and return 501, same as a fully-unimplemented tag's adapter.
type DealsAdapter struct {
	H *dealtransport.DealHandler
}

// ListDeals delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) ListDeals(w http.ResponseWriter, r *http.Request, params types.ListDealsParams) {
	a.H.ServeHTTP(w, r)
}

// CreateDeal delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) CreateDeal(w http.ResponseWriter, r *http.Request, params types.CreateDealParams) {
	a.H.ServeHTTP(w, r)
}

// GetDeal delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) GetDeal(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdateDeal delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) UpdateDeal(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateDealParams) {
	a.H.ServeHTTP(w, r)
}

// ArchiveDeal delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) ArchiveDeal(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// RestoreDeal delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) RestoreDeal(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RestoreDealParams) {
	a.H.ServeHTTP(w, r)
}

// AdvanceDeal delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) AdvanceDeal(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.AdvanceDealParams) {
	a.H.ServeHTTP(w, r)
}

// ListDealStakeholders delegates to the wired handler; see the struct doc comment above.
func (a *DealsAdapter) ListDealStakeholders(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// GetDealNextStep is unimplemented; see the struct doc comment above.
func (a *DealsAdapter) GetDealNextStep(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// AcceptDealNextStep is unimplemented; see the struct doc comment above.
func (a *DealsAdapter) AcceptDealNextStep(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.AcceptDealNextStepParams) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetDealKPISignals is unimplemented; see the struct doc comment above.
func (a *DealsAdapter) GetDealKPISignals(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// OverrideDealKPISignal is unimplemented; see the struct doc comment above.
func (a *DealsAdapter) OverrideDealKPISignal(w http.ResponseWriter, r *http.Request, idParam types.IdParam, key string) {
	w.WriteHeader(http.StatusNotImplemented)
}

// GetDealPeopleSignals is unimplemented; see the struct doc comment above.
func (a *DealsAdapter) GetDealPeopleSignals(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	w.WriteHeader(http.StatusNotImplemented)
}

// CorrectPersonSignal is unimplemented; see the struct doc comment above.
func (a *DealsAdapter) CorrectPersonSignal(w http.ResponseWriter, r *http.Request, idParam types.IdParam, personID openapi_types.UUID) {
	w.WriteHeader(http.StatusNotImplemented)
}
