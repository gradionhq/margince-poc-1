// Package relationships is the relationships domain module: employment and
// deal_stakeholder edge writes, plus partner-org kind reads (WS-E-a).
// This module.go provides the top-level Module type as a DI handle for
// application-layer wiring.
package relationships

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/relationships/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// Relationship is the typed edge (employment, deal_stakeholder, or partner-org kinds).
type Relationship = domain.Relationship

// RelationshipListFilter holds optional predicates for List.
type RelationshipListFilter = domain.RelationshipListFilter

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// RelationshipStore manages relationship rows.
type RelationshipStore = adapters.RelationshipStore

// ---------------------------------------------------------------------------
// Adapter constructor wrappers
// ---------------------------------------------------------------------------

// NewRelationshipStore returns a RelationshipStore backed by db.
func NewRelationshipStore(db *sql.DB) *RelationshipStore {
	return adapters.NewRelationshipStore(db)
}

// ---------------------------------------------------------------------------
// Module
// ---------------------------------------------------------------------------

// Module is the relationships module's dependency-injection handle.
type Module struct{}

// New returns a new relationships Module.
func New() *Module { return &Module{} }
