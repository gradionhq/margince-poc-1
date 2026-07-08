package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
)

// PeopleAdapter implements the People tag's slice of types.ServerInterface by
// delegating, unchanged, to the real PersonHandler cmd/api/routes.go already
// wires for /people. PersonHandler.ServeHTTP dispatches by request
// method+path itself, so every operation forwards to the same call — the
// typed path/query params oapi-codegen generates are intentionally unused
// (D10): the handler re-derives what it needs from r.
type PeopleAdapter struct {
	H *peopletransport.PersonHandler
}

// ListPeople delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) ListPeople(w http.ResponseWriter, r *http.Request, params types.ListPeopleParams) {
	a.H.ServeHTTP(w, r)
}

// CreatePerson delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) CreatePerson(w http.ResponseWriter, r *http.Request, params types.CreatePersonParams) {
	a.H.ServeHTTP(w, r)
}

// GetPerson delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) GetPerson(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// UpdatePerson delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) UpdatePerson(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdatePersonParams) {
	a.H.ServeHTTP(w, r)
}

// ArchivePerson delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) ArchivePerson(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}

// MergePerson delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) MergePerson(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.MergePersonParams) {
	a.H.ServeHTTP(w, r)
}

// RestorePerson delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) RestorePerson(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.RestorePersonParams) {
	a.H.ServeHTTP(w, r)
}

// GetPersonStrengthBreakdown delegates to the wired handler; see the struct doc comment above.
func (a *PeopleAdapter) GetPersonStrengthBreakdown(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.H.ServeHTTP(w, r)
}
