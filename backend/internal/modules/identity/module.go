// Package crmauth is the Tier-1 identity/RBAC module + Agent Seat Passport.
// This module.go re-exports all public types and functions from the adapters/
// subdirectory so external callers see an unchanged API (WS-E-a structural migration).
package crmauth

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/identity/adapters"
	"github.com/gradionhq/margince/backend/internal/platform/keyvault"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

// ErrForbidden is returned by AuthorizePerms on deny.
var ErrForbidden = adapters.ErrForbidden

// ErrScopeExceeded is returned by CheckScopeSubset when a requested scope is not
// carried by the grantor.
var ErrScopeExceeded = adapters.ErrScopeExceeded

// ErrNotFound is returned when a session/passport/etc. is not found.
var ErrNotFound = adapters.ErrNotFound

// ErrInvalidGrant is returned by AuthCodeStore.Consume on any failure that must fail closed.
var ErrInvalidGrant = adapters.ErrInvalidGrant

// CookieName is the session cookie name.
const CookieName = adapters.CookieName

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// PermissionEntry is one object's allowed actions with their row_scope.
type PermissionEntry = adapters.PermissionEntry

// ActionRule describes what a role can do for one action.
type ActionRule = adapters.ActionRule

// RolePermissions is the validated permissions map for a role.
type RolePermissions = adapters.RolePermissions

// Passport is an agent seat's scoped credential.
type Passport = adapters.Passport

// ---------------------------------------------------------------------------
// Session store
// ---------------------------------------------------------------------------

// SessionRecord is a loaded session row.
type SessionRecord = adapters.SessionRecord

// SessionStore manages session rows.
type SessionStore = adapters.SessionStore

// NewSessionStore returns a SessionStore backed by db.
func NewSessionStore(db *sql.DB) *SessionStore {
	return adapters.NewSessionStore(db)
}

// SHA256SumExported is exported for integration test use only.
func SHA256SumExported(s string) string { return adapters.SHA256SumExported(s) }

// ---------------------------------------------------------------------------
// Passport store
// ---------------------------------------------------------------------------

// PassportRecord is a loaded passport row.
type PassportRecord = adapters.PassportRecord

// PassportStore manages passport rows.
type PassportStore = adapters.PassportStore

// NewPassportStore returns a PassportStore backed by db.
func NewPassportStore(db *sql.DB) *PassportStore {
	return adapters.NewPassportStore(db)
}

// ---------------------------------------------------------------------------
// OAuth client store
// ---------------------------------------------------------------------------

// OAuthClientRecord is a registered DCR (RFC 7591) public client.
type OAuthClientRecord = adapters.OAuthClientRecord

// OAuthClientStore manages oauth_client rows.
type OAuthClientStore = adapters.OAuthClientStore

// NewOAuthClientStore returns an OAuthClientStore backed by db.
func NewOAuthClientStore(db *sql.DB) *OAuthClientStore {
	return adapters.NewOAuthClientStore(db)
}

// ---------------------------------------------------------------------------
// Auth code store
// ---------------------------------------------------------------------------

// AuthCodeRecord is the resolved grant behind a consumed authorization code.
type AuthCodeRecord = adapters.AuthCodeRecord

// AuthCodeStore manages one-time-use, short-lived PKCE authorization codes.
type AuthCodeStore = adapters.AuthCodeStore

// NewAuthCodeStore returns an AuthCodeStore backed by db.
func NewAuthCodeStore(db *sql.DB) *AuthCodeStore {
	return adapters.NewAuthCodeStore(db)
}

// PKCEChallengeS256 derives the RFC 7636 S256 code_challenge from a code_verifier.
func PKCEChallengeS256(verifier string) string {
	return adapters.PKCEChallengeS256(verifier)
}

// ---------------------------------------------------------------------------
// Connector secret store
// ---------------------------------------------------------------------------

// ConnectorSecretRecord is one loaded connector_secret row.
type ConnectorSecretRecord = adapters.ConnectorSecretRecord

// ConnectorSecretStore seals/unseals connector token material.
type ConnectorSecretStore = adapters.ConnectorSecretStore

// NewConnectorSecretStore returns a ConnectorSecretStore.
func NewConnectorSecretStore(db DBExec, provider keyvault.KeyProvider) *ConnectorSecretStore {
	return adapters.NewConnectorSecretStore(db, provider)
}

// ---------------------------------------------------------------------------
// Incumbent connection store
// ---------------------------------------------------------------------------

// IncumbentConnectionRecord is one loaded incumbent_connection row.
type IncumbentConnectionRecord = adapters.IncumbentConnectionRecord

// DBExec is satisfied by both *sql.Tx and *sql.DB.
type DBExec = adapters.DBExec

// IncumbentConnectionStore manages incumbent_connection rows.
type IncumbentConnectionStore = adapters.IncumbentConnectionStore

// NewIncumbentConnectionStore returns an IncumbentConnectionStore.
func NewIncumbentConnectionStore(db DBExec) *IncumbentConnectionStore {
	return adapters.NewIncumbentConnectionStore(db)
}

// ---------------------------------------------------------------------------
// Permission functions
// ---------------------------------------------------------------------------

// ValidatePermissions validates and parses a raw permissions JSONB map.
func ValidatePermissions(raw map[string]any) (RolePermissions, error) {
	return adapters.ValidatePermissions(raw)
}

// HashPassword returns a bcrypt hash of plain.
func HashPassword(plain string) (string, error) {
	return adapters.HashPassword(plain)
}

// VerifyPassword reports whether plain matches hash.
func VerifyPassword(hash, plain string) bool {
	return adapters.VerifyPassword(hash, plain)
}

// CheckScopeSubset returns ErrScopeExceeded if any requested scope is not
// present in grantorScopes.
func CheckScopeSubset(requested, grantorScopes []string) error {
	return adapters.CheckScopeSubset(requested, grantorScopes)
}

// AuthorizePerms checks whether perms allows action on object.
func AuthorizePerms(perms RolePermissions, object, action string) error {
	return adapters.AuthorizePerms(perms, object, action)
}

// LoadUserScopesFromPerms derives scope strings from a user's RolePermissions.
func LoadUserScopesFromPerms(perms RolePermissions) []string {
	return adapters.LoadUserScopesFromPerms(perms)
}
