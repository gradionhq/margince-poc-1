// Package crmagents is the overnight reconciliation pass's plumbing spine:
// context-assembler seam, no-guess gate, tier router, stager integration,
// approval-decided executor, and ranked/grouped batch assembler. This
// module.go re-exports domain/ports/app so external callers (the
// not-yet-built agent-runner) see one flat API — mirrors
// approvals/module.go's WS-E-a convention.
package crmagents

import (
	"context"
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// Proposal is one candidate overnight change.
type Proposal = domain.Proposal

// RoutedProposal pairs a gated Proposal with its derived tier.
type RoutedProposal = domain.RoutedProposal

// Fact is one provenance-stamped fact.
type Fact = domain.Fact

// AssembledView is the day's captured activity, provenance-stamped.
type AssembledView = domain.AssembledView

// RunState distinguishes normal/quiet/degraded runs.
type RunState = domain.RunState

// Run states.
const (
	RunNormal   = domain.RunNormal
	RunQuiet    = domain.RunQuiet
	RunDegraded = domain.RunDegraded
)

// ProposalGroup is one ActionType's ranked proposals.
type ProposalGroup = domain.ProposalGroup

// RunResult is the morning artifact.
type RunResult = domain.RunResult

// DBExec is satisfied by *sql.Tx and *sql.DB.
type DBExec = ports.DBExec

// EventEmitter is the outbox-write seam for this module's domain events.
type EventEmitter = ports.EventEmitter

// ConsumerGroup names the future live consumer group (OVN-EVT-1).
const ConsumerGroup = ports.ConsumerGroup

// ContextAssembler reads the day's captured activity, provenance-stamped.
type ContextAssembler = ports.ContextAssembler

// FixtureAssembler is the fixture-backed ContextAssembler this ticket ships.
type FixtureAssembler = ports.FixtureAssembler

// Produce turns an assembled view into raw proposals.
type Produce = ports.Produce

// StageFunc matches approvals/app.Stage's signature.
type StageFunc = ports.StageFunc

// Effector applies an approved or unconditionally-🟢 proposal's effect.
type Effector = ports.Effector

// ActorOvernight is the attribution string this module uses throughout.
const ActorOvernight = app.ActorOvernight

// TopicOvernightApplied is the domain event topic for a landed effect.
const TopicOvernightApplied = app.TopicOvernightApplied

// StageProposal stages a 🟡 RoutedProposal onto the approvals seam.
func StageProposal(ctx context.Context, tx DBExec, repo crmapprovals.Repository, stage StageFunc, p RoutedProposal, dryRunPreview json.RawMessage) (string, error) {
	return app.StageProposal(ctx, tx, repo, stage, p, dryRunPreview)
}

// ApplyGreen executes a 🟢 RoutedProposal's effect directly.
func ApplyGreen(ctx context.Context, tx DBExec, effector Effector, emitter EventEmitter, p RoutedProposal) (string, error) {
	return app.ApplyGreen(ctx, tx, effector, emitter, p)
}
