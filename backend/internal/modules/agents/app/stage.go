package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// StageProposal stages a 🟡 RoutedProposal onto the approvals seam via the
// injected StageFunc (production callers pass crmapprovals.Stage directly).
// ActionType is namespaced "overnight.<action>" so the approval-decided
// executor (executor.go) can dispatch on it later.
func StageProposal(ctx context.Context, tx ports.DBExec, repo crmapprovals.Repository, stage ports.StageFunc, p domain.RoutedProposal, dryRunPreview json.RawMessage) (string, error) {
	if p.Tier != mcp.TierYellow {
		return "", fmt.Errorf("agents stage: action_type %q is tier %v, not TierYellow", p.ActionType, p.Tier)
	}
	return stage(ctx, tx, repo, crmapprovals.StageInput{
		WorkspaceID:   p.WorkspaceID,
		ActionType:    "overnight." + p.ActionType,
		RequestedBy:   ActorOvernight,
		Payload:       p.Effect,
		DryRunPreview: dryRunPreview,
	})
}
