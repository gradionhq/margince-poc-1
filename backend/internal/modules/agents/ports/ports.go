// Package ports defines the agents module's injection seams. No package here
// ever reaches into another module's database directly — every seam is a
// narrow function/interface a caller injects.
package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// DBExec is satisfied by *sql.Tx and *sql.DB — reused from the approvals
// module's own seam so this module never redefines a second one.
type DBExec = crmapprovals.DBExec

// EventEmitter is the outbox-write seam for this module's own domain events
// (separate from approvals' own approval.requested/decided events) —
// reused, not redefined; it is topic-agnostic already.
type EventEmitter = crmapprovals.EventEmitter

// ConsumerGroup names the future Redis consumer group a live
// ContextAssembler will register under (OVN-EVT-1). Not wired to a real
// stream in this ticket — the fixture backs the interface below.
const ConsumerGroup = "cg:overnight-agent"

// ContextAssembler reads the day's captured activity through a
// provenance-stamped seam — never raw rows. A real event-bus-backed
// implementation is future work (T02-T05); this ticket's tests inject a
// fixture. RunPass (Task 7) calls Assemble directly — it never takes a
// pre-built view as a shortcut, so this seam is genuinely on the pass's
// critical path, not a defined-but-unused interface (OVN-EVT-1).
type ContextAssembler interface {
	Assemble(ctx context.Context, workspaceID string, since time.Time) (domain.AssembledView, error)
}

// FixtureAssembler is the fixture-backed ContextAssembler this ticket ships
// in place of a real event-bus-backed one (OVN-EVT-1's own acceptance note:
// no live capture emission is required here). Tests construct it directly;
// RunPass depends only on the ContextAssembler interface, so a future
// producer ticket's real implementation drops in unchanged.
type FixtureAssembler struct {
	View domain.AssembledView
	Err  error
}

// Assemble returns the fixture's canned view (or error) regardless of
// workspaceID/since — a real implementation will honor both.
func (f FixtureAssembler) Assemble(_ context.Context, _ string, _ time.Time) (domain.AssembledView, error) {
	return f.View, f.Err
}

// Produce turns an assembled view into the day's raw proposals (pre-gate,
// pre-tier). No real producer exists yet; a non-nil error signals a
// degraded run — Produce may still return a partial slice alongside it.
type Produce func(view domain.AssembledView) ([]domain.Proposal, error)

// StageFunc matches approvals/app.Stage's exact signature — production
// callers assign crmapprovals.Stage directly (`var _ StageFunc =
// crmapprovals.Stage`), never a second staging mechanism.
type StageFunc func(ctx context.Context, tx DBExec, repo crmapprovals.Repository, in crmapprovals.StageInput) (string, error)

// Effector applies an approved (or unconditionally 🟢) proposal's effect
// and returns a rollback handle. Injected — this module owns no domain
// table and never mutates one directly.
type Effector interface {
	Apply(ctx context.Context, tx DBExec, actionType string, payload json.RawMessage) (rollbackHandle string, err error)
}
