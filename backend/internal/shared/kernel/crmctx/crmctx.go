// Package crmctx is the Tier-0 governance seam: the acting principal, tenant,
// and agent passport carried in context. Every module reads the principal
// uniformly through this seam (ADR-0018). Dependency-free.
package crmctx

import "context"

// Principal is the acting identity for a request.
type Principal struct {
	UserID   string
	TenantID string
	IsAgent  bool
}

type ctxKey struct{}

// With attaches a Principal to ctx.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the Principal attached to ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
