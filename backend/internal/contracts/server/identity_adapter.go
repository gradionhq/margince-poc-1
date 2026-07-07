package server

import (
	"database/sql"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	identitytransport "github.com/gradionhq/margince/backend/internal/modules/identity/transport"
)

// IdentityAdapter implements the Identity tag's slice of types.ServerInterface
// by delegating to the real identitytransport handler funcs cmd/api/routes.go
// already builds for /workspaces, /auth, /me, /passports, /roles, /members.
// NewIdentityAdapter mirrors routes.go's construction exactly (same
// db/session/passport-store dependencies) so this stays in lockstep without
// building a second, divergent set of handlers (D10). The workspace/session
// bootstrap middleware routes.go layers on top (workspaceWrap, RequireAuth)
// is composition-layer plumbing, not part of "the handler", so it is not
// reproduced here — RequireManageMembers is kept because it gates the
// member-management handlers' own authorization outcome, not session setup.
type IdentityAdapter struct {
	CreateWorkspaceH  http.Handler
	LoginH            http.Handler
	LogoutH           http.Handler
	MeH               http.Handler
	CreatePassportH   http.Handler
	RevokePassportH   http.Handler
	ListRolesH        http.Handler
	ListMembersH      http.Handler
	AssignMemberRoleH http.Handler
	RevokeMemberRoleH http.Handler
}

// NewIdentityAdapter builds an IdentityAdapter from the same db/session/passport-store
// dependencies routes.go's registerObservabilityAndAuth already wires.
func NewIdentityAdapter(db *sql.DB, sessions *crmauth.SessionStore, passports *crmauth.PassportStore) *IdentityAdapter {
	return &IdentityAdapter{
		CreateWorkspaceH:  identitytransport.HandleCreateWorkspace(db),
		LoginH:            identitytransport.HandleLogin(db, sessions),
		LogoutH:           identitytransport.HandleLogout(sessions),
		MeH:               identitytransport.HandleMe(db),
		CreatePassportH:   identitytransport.HandleCreatePassport(db, passports),
		RevokePassportH:   identitytransport.HandleRevokePassport(passports),
		ListRolesH:        identitytransport.RequireManageMembers(db, identitytransport.HandleListRoles(db)),
		ListMembersH:      identitytransport.RequireManageMembers(db, identitytransport.HandleListMembers(db)),
		AssignMemberRoleH: identitytransport.RequireManageMembers(db, identitytransport.HandleAssignRole(db)),
		RevokeMemberRoleH: identitytransport.RequireManageMembers(db, identitytransport.HandleRevokeRole(db)),
	}
}

// CreateWorkspace delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	a.CreateWorkspaceH.ServeHTTP(w, r)
}

// Login delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) Login(w http.ResponseWriter, r *http.Request) {
	a.LoginH.ServeHTTP(w, r)
}

// Logout delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) Logout(w http.ResponseWriter, r *http.Request) {
	a.LogoutH.ServeHTTP(w, r)
}

// GetCurrentPrincipal delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) GetCurrentPrincipal(w http.ResponseWriter, r *http.Request) {
	a.MeH.ServeHTTP(w, r)
}

// CreatePassport delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) CreatePassport(w http.ResponseWriter, r *http.Request) {
	a.CreatePassportH.ServeHTTP(w, r)
}

// RevokePassport delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) RevokePassport(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	a.RevokePassportH.ServeHTTP(w, r)
}

// ListRoles delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) ListRoles(w http.ResponseWriter, r *http.Request) {
	a.ListRolesH.ServeHTTP(w, r)
}

// ListMembers delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) ListMembers(w http.ResponseWriter, r *http.Request) {
	a.ListMembersH.ServeHTTP(w, r)
}

// AssignMemberRole delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) AssignMemberRole(w http.ResponseWriter, r *http.Request, userID openapi_types.UUID, params types.AssignMemberRoleParams) {
	a.AssignMemberRoleH.ServeHTTP(w, r)
}

// RevokeMemberRole delegates to the wired handler; see the struct doc comment above.
func (a *IdentityAdapter) RevokeMemberRole(w http.ResponseWriter, r *http.Request, userID openapi_types.UUID, roleKey string) {
	a.RevokeMemberRoleH.ServeHTTP(w, r)
}
