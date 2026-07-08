package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	orgstransport "github.com/gradionhq/margince/backend/internal/modules/organizations/transport"
)

// OrganizationsAdapter implements the Organizations tag's slice of
// types.ServerInterface by delegating to the real OrganizationHandler
// cmd/api/routes.go already wires for /organizations. GetPartner/UpsertPartner
// are also tagged Organizations in crm.yaml but are implemented once, on
// PartnersAdapter (their primary tag), to avoid a duplicate method
// declaration across two embedded adapters in AllOperations.
type OrganizationsAdapter struct {
	H *orgstransport.OrganizationHandler
}

// ListOrganizations delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) ListOrganizations(w http.ResponseWriter, r *http.Request, params types.ListOrganizationsParams) {
	a.H.ServeHTTP(w, r)
}

// CreateOrganization delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) CreateOrganization(w http.ResponseWriter, r *http.Request, params types.CreateOrganizationParams) {
	a.H.ServeHTTP(w, r)
}

// GetOrganization delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) GetOrganization(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdateOrganization delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) UpdateOrganization(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateOrganizationParams) {
	a.H.ServeHTTP(w, r)
}

// ArchiveOrganization delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) ArchiveOrganization(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// MergeOrganization delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) MergeOrganization(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.MergeOrganizationParams) {
	a.H.ServeHTTP(w, r)
}

// RestoreOrganization delegates to the wired handler; see the struct doc comment above.
func (a *OrganizationsAdapter) RestoreOrganization(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RestoreOrganizationParams) {
	a.H.ServeHTTP(w, r)
}

// GetOrganizationHierarchyRollup is unimplemented (RD-T02/RD-WIRE-4 mints the
// contract only; the recursive-CTE roll-up query is out of scope) — returns
// 501, the same shape oapi-codegen's own types.Unimplemented stub uses.
func (a *OrganizationsAdapter) GetOrganizationHierarchyRollup(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.GetOrganizationHierarchyRollupParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
