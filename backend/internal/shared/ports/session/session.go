// Package session is the Tier-0 port for session and RBAC operations.
// It decouples platform/httpserver from modules/identity (AC-E3).
package session

import (
	"context"
	"errors"
	"fmt"

	crmctx "github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// CookieName is the session cookie name, shared by the middleware and the
// identity module's session store.
const CookieName = "crm_session"

// Verifier is the narrow interface httpserver's SessionMiddleware depends on.
// The concrete implementation lives in modules/identity/adapters and is wired
// at the composition root (cmd/api).
type Verifier interface {
	// LookupSession resolves a session cookie value to a Principal.
	// Returns (zero, false) if the token is invalid or expired.
	LookupSession(ctx context.Context, rawToken string) (crmctx.Principal, bool)
	// LookupPassport resolves a Bearer passport token to a Principal.
	// Returns (zero, false) if the token is invalid or expired.
	LookupPassport(ctx context.Context, rawToken string) (crmctx.Principal, bool)
}

// ---------------------------------------------------------------------------
// Pure RBAC helpers (moved from modules/identity/adapters — zero I/O).
// modules/identity/adapters re-exports these to preserve its existing API.
// ---------------------------------------------------------------------------

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

// knownObjects is the V1 object set for permission validation.
var knownObjects = map[string]bool{
	"person": true, "organization": true, "deal": true, "pipeline": true,
	"stage": true, "activity": true, "lead": true, "report": true,
	"passport": true, "workspace": true, "product": true, "invoice": true,
	"approval": true, "drafting_asset": true, "conversation_link": true,
	"deal_room":      true, // B-E08.6: deal-room publish access gate
	"automation":     true, // B-E15.4: automation CRUD endpoints
	"partner":        true,
	"relationship":   true, // T08: generic employment/deal_stakeholder edge CRUD
	"record_grant":   true, // GH-209 WS-B: record_grant sharing/manage_sharing gate
	"offer_template": true,
	"attachment":     true, // RD-T05: attachment CRUD RBAC gate
	"custom_field":   true, // CF-T03: custom-field definition CRUD gate
}

var knownRowScopes = map[string]bool{
	"own": true, "team": true, "all": true,
}

var knownActions = map[string]bool{
	"read": true, "create": true, "update": true, "archive": true,
	"delete": true, "export": true, "manage_members": true,
	"decide": true, "curate": true, "import": true,
	"publish": true,
}

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

// ErrForbidden is returned by AuthorizePerms on deny.
var ErrForbidden = errors.New("forbidden")

// ErrScopeExceeded is returned by CheckScopeSubset when a requested scope is not
// carried by the grantor.
var ErrScopeExceeded = errors.New("scope exceeded")

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
