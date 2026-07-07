package adapters

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// SessionVerifier satisfies ports/session.Verifier using the concrete
// SessionStore and PassportStore. Wired at the composition root (cmd/api).
type SessionVerifier struct {
	Sessions  *SessionStore
	Passports *PassportStore
}

// LookupSession resolves a session cookie value to a Principal.
// It also updates the session's idle expiry (Touch) as a side effect.
func (v *SessionVerifier) LookupSession(ctx context.Context, rawToken string) (crmctx.Principal, bool) {
	rec, err := v.Sessions.Lookup(ctx, rawToken)
	if err != nil {
		return crmctx.Principal{}, false
	}
	v.Sessions.Touch(ctx, rec.WorkspaceID, rec.ID)
	return crmctx.Principal{UserID: rec.UserID, TenantID: rec.WorkspaceID}, true
}

// LookupPassport resolves a Bearer passport token to a Principal.
func (v *SessionVerifier) LookupPassport(ctx context.Context, rawToken string) (crmctx.Principal, bool) {
	rec, err := v.Passports.Lookup(ctx, rawToken)
	if err != nil {
		return crmctx.Principal{}, false
	}
	return crmctx.Principal{UserID: rec.GrantedBy, TenantID: rec.WorkspaceID, IsAgent: true}, true
}
