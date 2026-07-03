package datasource

import "github.com/gradionhq/margince/backend/internal/shared/kernel/trust"

// TrustTier is re-exported from the Tier-0 kernel package crm/trust so the
// provider seam (datasource) and the dependency-free retrieval seam can both carry the
// tier without importing each other.
type TrustTier = trust.TrustTier

// The trust tiers re-exported from crm/trust.
const (
	TierT0 = trust.TierT0
	TierT1 = trust.TierT1
	TierT2 = trust.TierT2
)

// TrustWarning re-exports the crm/trust spotlight string for T2 content.
const TrustWarning = trust.TrustWarning

// TierProvider is an OPTIONAL capability a Provider may implement to
// declare the trust tier of the reads it serves. Kept off the 11-method
// Provider interface so native providers and test doubles need no
// change — the tool layer type-asserts via TierOf.
type TierProvider interface {
	ReadTier() TrustTier
}

// TierOf returns p's declared read tier if p implements TierProvider, else TierT1
// (the native/human default).
func TierOf(p any) TrustTier {
	if tp, ok := p.(TierProvider); ok {
		return tp.ReadTier()
	}
	return TierT1
}
