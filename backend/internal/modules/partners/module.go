// Package partners is the partner-program domain module (split from directory,
// WS-E-a, AC-E1). Domain types live in partners/domain/; storage adapters in
// partners/adapters/; application services in partners/app/; HTTP handlers in
// partners/transport/. This module.go re-exports the public surface so external
// callers can reference partners.Partner, partners.PartnerStore, etc.
package partners

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/partners/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/partners/domain"
	prov "github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// Partner is the 1:1 partner-program extension of an organization.
type Partner = domain.Partner

// PartnerListFilter holds optional predicates for List.
type PartnerListFilter = domain.PartnerListFilter

// ---------------------------------------------------------------------------
// Domain function wrappers
// ---------------------------------------------------------------------------

// NewPartner returns a Partner with a fresh ID, applied status, version 1, and copied provenance.
func NewPartner(organizationID string, p prov.Provenance) Partner {
	return domain.NewPartner(organizationID, p)
}

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// PartnerStore executes parameterized SQL against the partner table.
type PartnerStore = adapters.PartnerStore

// ---------------------------------------------------------------------------
// Adapter constructor wrappers
// ---------------------------------------------------------------------------

// NewPartnerStore returns a PartnerStore backed by db.
func NewPartnerStore(db *sql.DB) *PartnerStore {
	return adapters.NewPartnerStore(db)
}

// ---------------------------------------------------------------------------
// Module
// ---------------------------------------------------------------------------

// Module is the partners module's dependency-injection handle.
// Future tickets will add application services and port seams here as the
// module is progressively built out.
type Module struct{}

// New returns a new partners Module.
func New() *Module { return &Module{} }
