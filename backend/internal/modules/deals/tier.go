// Package deals provides the either-endpoint tier resolver for deal-stage
// transitions (DEAL-WIRE-4), migrated from the skeleton's superseded
// directory.terminalStageTier (target-semantic-only, dead code — zero
// callers) per the T12 spec. A transition is 🟡 when EITHER endpoint's
// semantic is terminal (won/lost): closing and reopening are both governed;
// open-to-open in either direction is 🟢. Resolving from *semantics* (never
// stage names) means renaming a stage cannot dodge the gate.
package deals

import "github.com/gradionhq/margince/backend/internal/shared/ports/mcp"

// Tier classifies a stage-transition's approval requirement level.
type Tier int

// Tier values classify the approval requirement for a deal-stage transition.
const (
	TierGreen  Tier = iota // no approval required
	TierYellow             // approval required for agent callers
)

func isTerminalSemantic(semantic string) bool {
	return semantic == "won" || semantic == "lost"
}

// ResolveTier returns the transition's tier from its FROM and TO stage semantics.
func ResolveTier(fromSemantic, toSemantic string) Tier {
	if isTerminalSemantic(fromSemantic) || isTerminalSemantic(toSemantic) {
		return TierYellow
	}
	return TierGreen
}

// ResolveDynamicTier adapts ResolveTier to the toolgate.RegisterResolver
// arg-map shape (WS-D-b, AC-D2): the advanceDeal x-mcp-tool's registered
// "target_stage_semantic" resolver reads from_semantic/to_semantic out of
// the diff fields the deal-advance handler already computes.
func ResolveDynamicTier(args map[string]any) mcp.RiskTier {
	from, _ := args["from_semantic"].(string)
	to, _ := args["to_semantic"].(string)
	if ResolveTier(from, to) == TierYellow {
		return mcp.TierYellow
	}
	return mcp.TierGreen
}
