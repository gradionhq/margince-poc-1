// Package adapters contains the identity module's pure types, functions,
// and storage adapters.
package adapters

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

// ErrForbidden is returned by AuthorizePerms on deny.
// Aliased from session port so that crmauth.ErrForbidden and
// session.ErrForbidden refer to the same sentinel.
var ErrForbidden = session.ErrForbidden

// ErrScopeExceeded is returned by CheckScopeSubset when a requested scope is not
// carried by the grantor.
var ErrScopeExceeded = session.ErrScopeExceeded

// ---------------------------------------------------------------------------
// RBAC types — canonical definitions now live in shared/ports/session.
// These aliases keep the existing crmauth API intact for all callers.
// ---------------------------------------------------------------------------

// PermissionEntry is one object's allowed actions with their row_scope.
type PermissionEntry = session.PermissionEntry

// ActionRule describes what a role can do for one action.
type ActionRule = session.ActionRule

// RolePermissions is the validated permissions map for a role.
type RolePermissions = session.RolePermissions

// ValidatePermissions validates and parses a raw permissions JSONB map.
func ValidatePermissions(raw map[string]any) (RolePermissions, error) {
	return session.ValidatePermissions(raw)
}

// AuthorizePerms checks whether perms allows action on object.
func AuthorizePerms(perms RolePermissions, object, action string) error {
	return session.AuthorizePerms(perms, object, action)
}

// LoadUserScopesFromPerms derives scope strings from a user's RolePermissions.
func LoadUserScopesFromPerms(perms RolePermissions) []string {
	var out []string
	for obj, entry := range perms {
		for action := range entry.Actions {
			out = append(out, action+":"+obj)
		}
	}
	return out
}

// HashPassword returns a bcrypt hash of plain.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword reports whether plain matches hash.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// CheckScopeSubset returns ErrScopeExceeded if any requested scope is not
// present in grantorScopes.
func CheckScopeSubset(requested, grantorScopes []string) error {
	have := make(map[string]bool, len(grantorScopes))
	for _, s := range grantorScopes {
		have[s] = true
	}
	for _, s := range requested {
		if !have[s] {
			return ErrScopeExceeded
		}
	}
	return nil
}

// Passport is an agent seat's scoped credential.
type Passport struct {
	ID          string
	WorkspaceID string
	GrantedBy   string
	Scopes      []string
}

// Has reports whether the passport carries a scope.
func (p Passport) Has(scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
