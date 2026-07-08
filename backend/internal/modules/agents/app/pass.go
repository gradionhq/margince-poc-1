package app

import (
	"context"
	"errors"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// PassInput carries every seam a future runner injects into one overnight
// pass invocation. The view always comes from Assembler.Assemble — there is
// no shortcut that lets a caller hand RunPass a pre-built view instead
// (OVN-EVT-1: the pass reads the day only through the injected seam).
type PassInput struct {
	WorkspaceID string
	Assembler   ports.ContextAssembler
	Since       time.Time
	Produce     ports.Produce
	Stage       ports.StageFunc
	Repo        crmapprovals.Repository
	Effector    ports.Effector
	Emitter     ports.EventEmitter
}

// RunPass is the pass a future agent-runner will call as a function — no
// scheduler, no retries, no admission gate here (a different, not-yet-built
// chapter). It reads the day through the injected ContextAssembler, gates,
// routes, stages 🟡 / applies 🟢, and returns the ranked/grouped batch (or
// an honest quiet/degraded state). An assembler failure degrades the run
// exactly like a producer failure — the run never blocks or fails core CRM
// (P4); it returns whatever survived, explicitly marked.
func RunPass(ctx context.Context, tx ports.DBExec, in PassInput) (domain.RunResult, error) {
	view, assemblerErr := in.Assembler.Assemble(ctx, in.WorkspaceID, in.Since)
	if assemblerErr != nil {
		return BuildBatch(nil, assemblerErr), nil
	}

	raw, producerErr := in.Produce(view)
	gated := GateProposals(raw)
	routed := RouteAll(gated)

	for _, p := range routed {
		switch p.Tier {
		case mcp.TierGreen:
			if _, err := ApplyGreen(ctx, tx, in.Effector, in.Emitter, p); err != nil {
				return domain.RunResult{}, err
			}
		default: // TierYellow (and any future TierDynamic result already floored to Yellow by RouteTier)
			if _, err := StageProposal(ctx, tx, in.Repo, in.Stage, p, nil); err != nil {
				// Stage always returns ErrRequiresApproval on success — that
				// is not a pass failure, it is the 🟡 lane working as
				// designed. Only an unexpected error (not
				// ErrRequiresApproval) aborts the pass.
				if !errors.Is(err, apperrors.ErrRequiresApproval) {
					return domain.RunResult{}, err
				}
			}
		}
	}

	return BuildBatch(routed, producerErr), nil
}
