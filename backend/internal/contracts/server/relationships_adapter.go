package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	dealtransport "github.com/gradionhq/margince/backend/internal/modules/directory/transport"
)

// RelationshipsAdapter implements the Relationships tag's slice of
// types.ServerInterface by delegating to the real RelationshipHandler
// cmd/api/routes.go already wires for /relationships.
type RelationshipsAdapter struct {
	H *dealtransport.RelationshipHandler
}

// ListRelationships delegates to the wired handler; see the struct doc comment above.
func (a *RelationshipsAdapter) ListRelationships(w http.ResponseWriter, r *http.Request, params types.ListRelationshipsParams) {
	a.H.ServeHTTP(w, r)
}

// CreateRelationship delegates to the wired handler; see the struct doc comment above.
func (a *RelationshipsAdapter) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	a.H.ServeHTTP(w, r)
}

// UpdateRelationship delegates to the wired handler; see the struct doc comment above.
func (a *RelationshipsAdapter) UpdateRelationship(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// ArchiveRelationship delegates to the wired handler; see the struct doc comment above.
func (a *RelationshipsAdapter) ArchiveRelationship(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
