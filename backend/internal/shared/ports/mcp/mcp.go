// Package mcp is the Tier-0 governed-tool seam (ADR-0013, one governed surface).
package mcp

import (
	"context"
	"encoding/json"
	"errors"
)

// RiskTier is the ONE risk-tier vocabulary for tool autonomy (ADR-0026): the line
// is reversibility. The 🟡 floor is tighten-only — a TierYellow tool can never be
// re-declared (or dynamically resolved) down to TierGreen.
type RiskTier int

// Risk tiers in the autonomy vocabulary, ordered by reversibility.
const (
	TierGreen   RiskTier = iota // 🟢 reversible: the agent acts alone
	TierYellow                  // 🟡 mutating/sending: held behind a human approval gate
	TierDynamic                 // resolved per-call by a TierResolver (floor is 🟡)
)

// Tier is the deprecated spelling retained for the static Tool.Tier() surface
// (and the crm-gen tool template). It, together with Green and Yellow, ALIASES
// into the single RiskTier vocabulary above — one type, one set of values — not
// a second enum. New code uses RiskTier / TierGreen / TierYellow directly.
type Tier = RiskTier

// Deprecated tier-value spellings aliased onto the RiskTier vocabulary.
const (
	Green  = TierGreen
	Yellow = TierYellow
)

// Tool is one governed MCP tool.
type Tool interface {
	Name() string
	Tier() RiskTier
	Invoke(ctx context.Context, args []byte) ([]byte, error)
}

// TierResolver resolves a TierDynamic spec to a concrete tier from call args.
type TierResolver func(args []byte) RiskTier

// ToolSpec is the governed declaration of a tool (B-EP06.8). It is data, not
// behavior — the admission gate reads it; the registry holds the runnable Tool.
type ToolSpec struct {
	Name        string
	Version     string
	Scope       []string
	Tier        RiskTier
	InputSchema json.RawMessage
	OpenAPIOp   string // logical OpenAPI family this tool maps to
	Egress      bool   // true if the tool sends data outside the tenant boundary
	Resolver    TierResolver
}

// Validate enforces ToolSpec well-formedness. A TierDynamic spec without a
// resolver is a programming error: the gate could not resolve its tier.
func (s ToolSpec) Validate() error {
	if s.Name == "" {
		return errors.New("mcp: ToolSpec.Name is required")
	}
	if s.Version == "" {
		return errors.New("mcp: ToolSpec.Version is required")
	}
	if s.Tier == TierDynamic && s.Resolver == nil {
		return errors.New("mcp: TierDynamic ToolSpec requires a non-nil TierResolver")
	}
	return nil
}

// ResolveTier returns the concrete tier for a call. For TierDynamic it consults
// the resolver (Validate guarantees non-nil), then CLAMPS the result to a concrete
// tier with a 🟡 floor: the resolver may resolve a reversible call to 🟢, but the
// ONLY value that escapes the gate is a clean TierGreen. Anything else — TierYellow,
// the un-resolved TierDynamic sentinel, or any out-of-range value a buggy or
// adversarial resolver hands back — floors to 🟡. So a resolver that mis-classifies
// a terminal (irreversible) action as anything but a clean 🟢 still routes through
// the approval gate; the floor "can never relax to 🟢" is enforced in code here, at
// the single tier-resolution choke point, not left to per-resolver discipline.
func (s ToolSpec) ResolveTier(args []byte) RiskTier {
	if s.Tier == TierDynamic && s.Resolver != nil {
		if s.Resolver(args) == TierGreen {
			return TierGreen
		}
		return TierYellow
	}
	return s.Tier
}

var registry []Tool

// Register adds t to the process-wide MCP tool registry.
func Register(t Tool) { registry = append(registry, t) }

// All returns all registered MCP tools.
func All() []Tool { return registry }
