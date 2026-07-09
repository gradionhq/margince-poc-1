// Package domain contains pure overnight-agent domain types and no
// database/sql, no net/http — mirrors approvals/domain's purity convention.
package domain

import (
	"encoding/json"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// Proposal is one candidate overnight change: an action, a target, and the
// evidence a human (or the narrow 🟢 lane) needs to judge it. ActionType is
// the ONE field a tier is ever derived from (app/tier.go); it also
// namespaces the staged approval_item as "overnight.<ActionType>" so the
// approval-decided executor can dispatch on it (app/executor.go). Nothing
// here lets a caller set a tier directly — see RoutedProposal.
type Proposal struct {
	WorkspaceID  string
	ActionType   string          // e.g. "log_link", "send", "close-deal" — D4 vocabulary or a fixture-declared green name
	TargetEntity string          // e.g. "deal:<id>", "activity:<id>" — what this proposal concerns
	Effect       json.RawMessage // the proposed effect payload, approvals-shaped
	Evidence     string          // non-empty snippet a human can read; GATE-AI-1
	Confidence   *float64        // nil means "missing" — never a 0.0 sentinel; GATE-AI-1
	Source       string          // resolvable provenance reference; GATE-AI-1
	EventTopic   string          // overrides the emitted event topic for this proposal; empty falls back to TopicOvernightApplied
}

// RoutedProposal pairs a gated Proposal with its derived tier — the ONLY
// way a Proposal ever acquires a tier (app/tier.go.RouteTier is the only
// producer of this type).
type RoutedProposal struct {
	Proposal
	Tier mcp.RiskTier
}

// Fact is one provenance-stamped fact the context-assembler seam surfaces —
// never a raw row.
type Fact struct {
	EntityType string
	EntityID   string
	Detail     string
	Source     string
	CapturedBy string
	OccurredAt time.Time
}

// AssembledView is the day's captured activity, provenance-stamped, exactly
// as the context-assembler seam returns it — the only way the pass sees
// "the day" (OVN-EVT-1). WindowStart/WindowEnd bound what "since the last
// run" covered.
type AssembledView struct {
	WorkspaceID string
	WindowStart time.Time
	WindowEnd   time.Time
	Facts       []Fact
}

// RunState distinguishes an honest empty/degraded run from a normal batch —
// never padding, never a silently-partial pass.
type RunState string

// RunNormal/RunQuiet/RunDegraded distinguish the three honest pass outcomes.
const (
	RunNormal   RunState = "normal"   // a non-empty ranked, grouped batch
	RunQuiet    RunState = "quiet"    // zero proposals survived the gate — an honest "nothing needed"
	RunDegraded RunState = "degraded" // the assembler or a producer partially failed
)

// ProposalGroup is one ActionType's proposals, ranked by confidence
// descending (OVN-AC-3).
type ProposalGroup struct {
	ActionType string
	Items      []RoutedProposal
}

// RunResult is the morning artifact.
type RunResult struct {
	State          RunState
	Groups         []ProposalGroup
	DegradedReason string // non-empty only when State == RunDegraded
}
