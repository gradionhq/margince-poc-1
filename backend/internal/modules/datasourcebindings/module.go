// Package datasourcebindings provides the datasource.Provider implementation that bridges
// the CRM entity stores (people, deals, activities, leads) to the datasource.Provider seam
// (ADR-0013). The module.go re-exports the public types and constructor from adapters/ so
// external callers see a single, stable import path (WS-E-a structural migration).
package datasourcebindings

import (
	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/ports"
)

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// DatasourceProvider wraps the CRM entity stores and implements datasource.Provider.
type DatasourceProvider = adapters.DatasourceProvider

// ---------------------------------------------------------------------------
// Port interface aliases (for use by callers wiring concrete store adapters)
// ---------------------------------------------------------------------------

// PersonStorer is the persistence seam for person records.
type PersonStorer = ports.PersonStorer

// OrgStorer is the persistence seam for organization records.
type OrgStorer = ports.OrgStorer

// DealStorer is the persistence seam for deal records.
type DealStorer = ports.DealStorer

// ActivityStorer is the persistence seam for activity records.
type ActivityStorer = ports.ActivityStorer

// LeadStorer is the persistence seam for lead records.
type LeadStorer = ports.LeadStorer

// ---------------------------------------------------------------------------
// Adapter constructor wrapper
// ---------------------------------------------------------------------------

// NewDatasourceProvider constructs a DatasourceProvider backed by the given stores.
// The concrete store types from the entity modules must satisfy the *Storer interfaces.
func NewDatasourceProvider(
	workspaceID string,
	persons PersonStorer,
	orgs OrgStorer,
	deals DealStorer,
	activities ActivityStorer,
	leads LeadStorer,
) *DatasourceProvider {
	return adapters.NewDatasourceProvider(workspaceID, persons, orgs, deals, activities, leads)
}
