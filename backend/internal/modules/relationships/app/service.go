// Package app contains the relationships module's application services (WS-E-a scaffold).
package app

import (
	"github.com/gradionhq/margince/backend/internal/modules/relationships/ports"
)

// RelationshipService is a thin scaffold for relationship business logic.
// Future tickets will add business rules here as the module is built out.
type RelationshipService struct {
	store ports.Store
}

// NewRelationshipService returns a RelationshipService backed by store.
func NewRelationshipService(store ports.Store) *RelationshipService {
	return &RelationshipService{store: store}
}
