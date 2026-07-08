package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// overnightNamespace prefixes every StageInput.ActionType this module
// stages (stage.go) — HandleDecided uses it to recognize its own items.
const overnightNamespace = "overnight."

// DecidedEventPayload mirrors approvals/app.Decider's emitted
// approval.decided payload shape ({"decision":..., "item_id":...}). This
// module has no live consumer of the real Redis stream in this ticket (the
// not-yet-built agent-runner owns that wiring) — HandleDecided is the exact
// function shape that consumer will call once it exists.
type DecidedEventPayload struct {
	Decision string `json:"decision"`
	ItemID   string `json:"item_id"`
}

// HandleDecided consumes one approval.decided event. If the referenced
// item's ActionType is not namespaced "overnight.", it belongs to some
// other module — no-op. On "approved" or "modified" it executes the
// (possibly edited) effect via the injected Effector, then writes one
// audit row + emits one domain event attributed to ActorOvernight
// (OVN-AC-8/GATE-CORE-5) — separate from the approvals module's own
// "approve"/"modify" audit row, which is attributed to the human. On
// "rejected" (or any other decision), it does nothing outward. repo.Get
// runs before the tx is scoped — mirrors Decider.Approve's own order
// (decision.go): the base connection reads the item first, then everything
// downstream (effector, audit, emit) runs inside the workspace-scoped
// role/GUC audit_log/event_outbox's RLS requires.
func HandleDecided(ctx context.Context, tx ports.DBExec, repo crmapprovals.Repository, effector ports.Effector, emitter ports.EventEmitter, payload DecidedEventPayload) error {
	item, err := repo.Get(ctx, tx, payload.ItemID)
	if err != nil {
		return fmt.Errorf("agents handle decided: get %s: %w", payload.ItemID, err)
	}
	if !strings.HasPrefix(item.ActionType, overnightNamespace) {
		return nil
	}
	if payload.Decision != "approved" && payload.Decision != "modified" {
		return nil
	}

	if err := database.SetWorkspaceScope(ctx, tx, item.WorkspaceID); err != nil {
		return fmt.Errorf("agents handle decided: guc: %w", err)
	}

	actionType := strings.TrimPrefix(item.ActionType, overnightNamespace)
	rollbackHandle, err := effector.Apply(ctx, tx, actionType, item.Payload)
	if err != nil {
		return fmt.Errorf("agents handle decided: effector: %w", err)
	}

	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID: item.WorkspaceID,
		ActorType:   "agent",
		ActorID:     ActorOvernight,
		Action:      "update",
		EntityType:  "approval_item",
		EntityID:    &item.ID,
		After:       map[string]any{keyActionType: actionType, "decision": payload.Decision, keyRollbackHandle: rollbackHandle},
	}); err != nil {
		return fmt.Errorf("agents handle decided: audit: %w", err)
	}

	out, _ := json.Marshal(map[string]any{keyActionType: actionType, "item_id": item.ID, keyRollbackHandle: rollbackHandle})
	if err := emitter.Emit(ctx, tx, TopicOvernightApplied, item.WorkspaceID, item.ID, out); err != nil {
		return fmt.Errorf("agents handle decided: emit: %w", err)
	}
	return nil
}
