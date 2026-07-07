// Package organizations is the organizations domain module (WS-E-a).
// This module.go provides the top-level Module type as a DI handle and
// re-exports the key types and constructors external callers need.
// The domain types, store adapters, and HTTP handler live in:
//   - organizations/domain — Organization, OrganizationDomain, OrgStrengthBlock, OrgListFilter
//   - organizations/adapters — OrgStore (CRUD + merge + strength aggregation)
//   - organizations/ports — OrgStorer interface
//   - organizations/transport — OrganizationHandler (HTTP)
//   - organizations/app — OrgService (scaffold)
package organizations

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
)

// Type aliases for external callers so they can refer to org types via the
// module package rather than reaching into sub-packages directly.
type Organization = domain.Organization
type OrgStore = adapters.OrgStore
type OrgListFilter = domain.OrgListFilter

// NewOrgStore is a convenience constructor for external wiring.
func NewOrgStore(db *sql.DB) *OrgStore { return adapters.NewOrgStore(db) }

// Module is the organizations module's dependency-injection handle.
// Future tickets will add application services and wire the full module here.
type Module struct{}

// New returns a new organizations Module.
func New() *Module { return &Module{} }
