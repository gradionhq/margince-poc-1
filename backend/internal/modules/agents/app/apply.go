package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// ApplyGreen executes a 🟢 RoutedProposal directly through the injected
// Effector — reversible only, and only when the effect returns a
// non-empty rollback handle. Writes exactly one audit row + emits exactly
// one domain event in the same tx (GATE-CORE-5), attributed to
// ActorOvernight (OVN-AC-8). Scopes the tx to p.WorkspaceID first —
// audit_log/event_outbox are FORCE ROW LEVEL SECURITY, deny-on-unset
// (mirrors approvals/app.Stage's own first step).
func ApplyGreen(ctx context.Context, tx ports.DBExec, effector ports.Effector, emitter ports.EventEmitter, p domain.RoutedProposal) (rollbackHandle string, err error) {
	if p.Tier != mcp.TierGreen {
		return "", fmt.Errorf("agents apply green: action_type %q is tier %v, not TierGreen", p.ActionType, p.Tier)
	}
	if err := database.SetWorkspaceScope(ctx, tx, p.WorkspaceID); err != nil {
		return "", fmt.Errorf("agents apply green: guc: %w", err)
	}
	rollbackHandle, err = effector.Apply(ctx, tx, p.ActionType, p.Effect)
	if err != nil {
		return "", fmt.Errorf("agents apply green: effector: %w", err)
	}
	if rollbackHandle == "" {
		return "", fmt.Errorf("agents apply green: action_type %q effect carries no rollback handle", p.ActionType)
	}

	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID: p.WorkspaceID,
		ActorType:   "agent",
		ActorID:     ActorOvernight,
		Action:      "update",
		EntityType:  entityTypeFromTarget(p.TargetEntity),
		After:       map[string]any{"action_type": p.ActionType, "target": p.TargetEntity, "rollback_handle": rollbackHandle, "tier": "green"},
	}); err != nil {
		return "", fmt.Errorf("agents apply green: audit: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{"action_type": p.ActionType, "target": p.TargetEntity, "rollback_handle": rollbackHandle})
	if err := emitter.Emit(ctx, tx, TopicOvernightApplied, p.WorkspaceID, entityIDFromTarget(p.TargetEntity), payload); err != nil {
		return "", fmt.Errorf("agents apply green: emit: %w", err)
	}
	return rollbackHandle, nil
}
