package app

import (
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// d4FloorActionTypes are threat-model.md#D4's always-🟡 action names,
// declared as static TierYellow ToolSpecs (never TierDynamic) — the floor
// is strongest as a static declaration a resolver can't misconfigure
// (A34/ADR-0026, "never re-tiered, not by configuration").
var d4FloorActionTypes = map[string]bool{
	"send":       true,
	"outbound":   true,
	"archive":    true,
	"merge":      true,
	"disqualify": true,
	"close-deal": true,
	"enrich":     true,
}

// greenActionTypes are the ONLY action types this spine declares
// unconditionally reversible (a static TierGreen ToolSpec). Every other
// action type — including one this ticket has never heard of — defaults
// to the 🟡 floor below. A future producer with a conditionally-reversible
// action type is the first real caller for mcp.TierDynamic + a
// TierResolver; not needed by any action type this ticket declares.
var greenActionTypes = map[string]bool{
	"log_link":                   true, // fixture: a reversible activity-to-record link
	"close-date-auto-apply":      true, // FCAST-FORM-3 AUTO_APPLY: reversible, rollback-carrying
	"close-date-provisional-set": true, // always-green invariant placeholder (OVN-AC-1)
}

// toolSpecFor is the ONE place a Proposal's tier is decided. The D4 map is
// checked FIRST and unconditionally, so even an accidental future overlap
// between the two maps can never let a D4 name resolve green — this
// ordering is the structural guarantee TestRouteTier_D4FloorNamesAlwaysYellow
// proves.
func toolSpecFor(actionType string) mcp.ToolSpec {
	if d4FloorActionTypes[actionType] {
		return mcp.ToolSpec{Name: actionType, Version: "v1", Tier: mcp.TierYellow}
	}
	if greenActionTypes[actionType] {
		return mcp.ToolSpec{Name: actionType, Version: "v1", Tier: mcp.TierGreen}
	}
	return mcp.ToolSpec{Name: actionType, Version: "v1", Tier: mcp.TierYellow} // default-deny
}

// RouteTier derives a Proposal's tier from its ActionType alone — never
// caller-set (OVN-AC-2/GATE-AI-7). The only production path to a
// RoutedProposal.
func RouteTier(p domain.Proposal) domain.RoutedProposal {
	tier := toolSpecFor(p.ActionType).ResolveTier(p.Effect)
	return domain.RoutedProposal{Proposal: p, Tier: tier}
}

// RouteAll routes a batch of gated proposals, in order.
func RouteAll(in []domain.Proposal) []domain.RoutedProposal {
	out := make([]domain.RoutedProposal, 0, len(in))
	for _, p := range in {
		out = append(out, RouteTier(p))
	}
	return out
}
