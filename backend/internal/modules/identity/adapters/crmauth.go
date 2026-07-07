// Package adapters contains the identity module's pure types, functions,
// and storage adapters.
package adapters

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrForbidden is returned by AuthorizePerms on deny.
var ErrForbidden = errors.New("forbidden")

// ErrScopeExceeded is returned by CheckScopeSubset when a requested scope is not
// carried by the grantor.
var ErrScopeExceeded = errors.New("scope exceeded")

// knownObjects is the V1 object set for permission validation.
var knownObjects = map[string]bool{
	"person": true, "organization": true, "deal": true, "pipeline": true,
	"stage": true, "activity": true, "lead": true, "report": true,
	"passport": true, "workspace": true, "product": true, "invoice": true,
	"approval": true, "drafting_asset": true, "conversation_link": true,
	"deal_room":    true, // B-E08.6: deal-room publish access gate
	"automation":   true, // B-E15.4: automation CRUD endpoints
	"partner":      true,
	"relationship": true, // T08: generic employment/deal_stakeholder edge CRUD
}

// knownRowScopes is the valid row_scope token set.
var knownRowScopes = map[string]bool{
	"own": true, "team": true, "all": true,
}

// knownActions is the V1 action set.
var knownActions = map[string]bool{
	"read": true, "create": true, "update": true, "archive": true,
	"delete": true, "export": true, "manage_members": true,
	"decide": true, "curate": true, "import": true,
	"publish": true, // B-E08.6: deal-room publish action
}

// PermissionEntry is one object's allowed actions with their row_scope.
type PermissionEntry struct {
	Actions map[string]ActionRule // action -> rule
}

// ActionRule describes what a role can do for one action.
type ActionRule struct {
	RowScope  string            // own | team | all
	FieldMask map[string]string // field -> read|write|hidden; nil = all allowed
}

// RolePermissions is the validated permissions map for a role.
type RolePermissions map[string]PermissionEntry // object -> entry

// ValidatePermissions validates and parses a raw permissions JSONB map.
func ValidatePermissions(raw map[string]any) (RolePermissions, error) {
	out := make(RolePermissions)
	for obj, v := range raw {
		if !knownObjects[obj] {
			return nil, fmt.Errorf("unknown object %q in permissions", obj)
		}
		actions, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("permissions[%q] must be an object", obj)
		}
		entry := PermissionEntry{Actions: make(map[string]ActionRule)}
		for action, av := range actions {
			if !knownActions[action] {
				return nil, fmt.Errorf("unknown action %q for object %q", action, obj)
			}
			ar, ok := av.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("permissions[%q][%q] must be an object", obj, action)
			}
			rs, _ := ar["row_scope"].(string)
			if !knownRowScopes[rs] {
				return nil, fmt.Errorf("invalid row_scope %q for %q.%q", rs, obj, action)
			}
			entry.Actions[action] = ActionRule{RowScope: rs}
		}
		out[obj] = entry
	}
	return out, nil
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
			return fmt.Errorf("%w: %q not in grantor scopes", ErrScopeExceeded, s)
		}
	}
	return nil
}

// AuthorizePerms checks whether perms allows action on object.
func AuthorizePerms(perms RolePermissions, object, action string) error {
	entry, ok := perms[object]
	if !ok {
		return ErrForbidden
	}
	if _, ok := entry.Actions[action]; !ok {
		return ErrForbidden
	}
	return nil
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
