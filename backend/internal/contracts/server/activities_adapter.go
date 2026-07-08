package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	actstransport "github.com/gradionhq/margince/backend/internal/modules/activities/transport"
)

// ActivitiesAdapter implements the Activities tag's slice of
// types.ServerInterface by delegating to the real ActivityHandler
// cmd/api/routes.go already wires for /activities. DraftEmail/SendEmail are
// also tagged Activities in crm.yaml but are declared once, on AIAdapter
// (their other tag), to avoid a duplicate method declaration across two
// embedded adapters in AllOperations — neither has a wired handler in this
// pruned tree, so AIAdapter stubs them 501 like the rest of its tag.
type ActivitiesAdapter struct {
	H *actstransport.ActivityHandler
}

// ListActivities delegates to the wired handler; see the struct doc comment above.
func (a *ActivitiesAdapter) ListActivities(w http.ResponseWriter, r *http.Request, params types.ListActivitiesParams) {
	a.H.ServeHTTP(w, r)
}

// LogActivity delegates to the wired handler; see the struct doc comment above.
func (a *ActivitiesAdapter) LogActivity(w http.ResponseWriter, r *http.Request, params types.LogActivityParams) {
	a.H.ServeHTTP(w, r)
}

// GetActivity delegates to the wired handler; see the struct doc comment above.
func (a *ActivitiesAdapter) GetActivity(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdateActivity delegates to the wired handler; see the struct doc comment above.
func (a *ActivitiesAdapter) UpdateActivity(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateActivityParams) {
	a.H.ServeHTTP(w, r)
}

// ArchiveActivity delegates to the wired handler; see the struct doc comment above.
func (a *ActivitiesAdapter) ArchiveActivity(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
