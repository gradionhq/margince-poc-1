package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
	"github.com/gradionhq/margince/backend/internal/modules/approvals/ports"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// Internal audit-field constants.
const (
	actorTypeHuman   = "human"
	actorTypeSystem  = "system"
	entityApproval   = "approval_item"
	actionModify     = "modify"
	decisionKey      = "decision"
	itemIDKey        = "item_id"
	decisionModified = "modified"
)

// Decider executes approval decisions (approve / reject / modify).
type Decider struct {
	Repo       ports.Repository
	Datasource datasource.Provider
	Emitter    ports.EventEmitter
	// ResolveYellow reports whether (action_type, payload) re-resolves to the 🟡 tier.
	ResolveYellow func(actionType string, payload json.RawMessage) bool
}

// Approve claims a pending approval_item and executes the underlying datasource action.
func (d Decider) Approve(ctx context.Context, tx ports.DBExec, id, approverID string) error {
	item, err := d.Repo.Get(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("decider approve get: %w", err)
	}
	if item.Status != domain.StatusPending {
		return fmt.Errorf("decider approve: item %s is not pending (status=%s)", id, item.Status)
	}

	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.workspace_id', $1, true)`, item.WorkspaceID); err != nil {
		return fmt.Errorf("decider approve guc: %w", err)
	}

	if err := d.Repo.Transition(ctx, tx, id, domain.StatusApproved, approverID); err != nil {
		return fmt.Errorf("decider approve transition: %w", err)
	}

	if err := d.execAction(ctx, item, item.Payload); err != nil {
		return fmt.Errorf("decider approve exec: %w", err)
	}

	authRule := "mcp.approve"
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       item.WorkspaceID,
		ActorType:         actorTypeHuman,
		ActorID:           approverID,
		Action:            "approve",
		EntityType:        entityApproval,
		EntityID:          &id,
		After:             map[string]any{decisionKey: "approved"},
		AuthorizationRule: &authRule,
	}); err != nil {
		return fmt.Errorf("decider approve audit: %w", err)
	}

	if d.Emitter != nil {
		p, _ := json.Marshal(map[string]string{decisionKey: "approved", itemIDKey: id})
		if err := d.Emitter.Emit(ctx, tx, topicApprovalDecided, item.WorkspaceID, id, p); err != nil {
			return fmt.Errorf("decider approve emit: %w", err)
		}
	}

	return nil
}

// topicApprovalDecided is the event topic for approval decisions.
const topicApprovalDecided = "approval.decided"

// Reject transitions a pending approval_item to rejected.
func (d Decider) Reject(ctx context.Context, tx ports.DBExec, id, approverID, reason string) error {
	item, err := d.Repo.Get(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("decider reject get: %w", err)
	}
	if item.Status != domain.StatusPending {
		return fmt.Errorf("decider reject: item %s is not pending (status=%s)", id, item.Status)
	}

	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.workspace_id', $1, true)`, item.WorkspaceID); err != nil {
		return fmt.Errorf("decider reject guc: %w", err)
	}

	if err := d.Repo.Transition(ctx, tx, id, domain.StatusRejected, approverID); err != nil {
		return fmt.Errorf("decider reject transition: %w", err)
	}

	authRule := "mcp.reject"
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       item.WorkspaceID,
		ActorType:         actorTypeHuman,
		ActorID:           approverID,
		Action:            "reject",
		EntityType:        entityApproval,
		EntityID:          &id,
		After:             map[string]any{decisionKey: "rejected", "reason": reason},
		AuthorizationRule: &authRule,
	}); err != nil {
		return fmt.Errorf("decider reject audit: %w", err)
	}

	if d.Emitter != nil {
		p, _ := json.Marshal(map[string]string{decisionKey: "rejected", itemIDKey: id})
		if err := d.Emitter.Emit(ctx, tx, topicApprovalDecided, item.WorkspaceID, id, p); err != nil {
			return fmt.Errorf("decider reject emit: %w", err)
		}
	}

	return nil
}

// Modify transitions a pending approval_item to modified, runs the gate, then
// executes the datasource action with editedPayload.
func (d Decider) Modify(ctx context.Context, tx ports.DBExec, id, approverID string, editedPayload json.RawMessage, gate ports.AdmitFunc) error {
	item, err := d.Repo.Get(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("decider modify get: %w", err)
	}
	if item.Status != domain.StatusPending {
		return fmt.Errorf("decider modify: item %s is not pending (status=%s)", id, item.Status)
	}

	if err := gate(ctx, approverID, item.ActionType, editedPayload); err != nil {
		return fmt.Errorf("decider modify gate: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.workspace_id', $1, true)`, item.WorkspaceID); err != nil {
		return fmt.Errorf("decider modify guc: %w", err)
	}

	if d.ResolveYellow != nil && d.ResolveYellow(item.ActionType, editedPayload) {
		return d.reStageModified(ctx, tx, item, id, approverID, editedPayload)
	}

	if err := d.Repo.Transition(ctx, tx, id, domain.StatusModified, approverID); err != nil {
		return fmt.Errorf("decider modify transition: %w", err)
	}

	if err := d.execAction(ctx, item, editedPayload); err != nil {
		return fmt.Errorf("decider modify exec: %w", err)
	}

	authRuleDelta := "mcp.modify.human_delta"
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       item.WorkspaceID,
		ActorType:         actorTypeHuman,
		ActorID:           approverID,
		Action:            actionModify,
		EntityType:        entityApproval,
		EntityID:          &id,
		Before:            map[string]any{"original_payload": string(item.Payload)},
		After:             map[string]any{"edited_payload": string(editedPayload), decisionKey: decisionModified},
		AuthorizationRule: &authRuleDelta,
	}); err != nil {
		return fmt.Errorf("decider modify audit delta: %w", err)
	}

	authRuleProposal := "mcp.modify.original_proposal"
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       item.WorkspaceID,
		ActorType:         actorTypeHuman,
		ActorID:           approverID,
		Action:            actionModify,
		EntityType:        entityApproval,
		EntityID:          &id,
		After:             map[string]any{"original_action_type": item.ActionType, "requested_by": item.RequestedBy},
		AuthorizationRule: &authRuleProposal,
	}); err != nil {
		return fmt.Errorf("decider modify audit proposal: %w", err)
	}

	return nil
}

// reStageModified handles a modify whose edited payload re-resolves to 🟡.
func (d Decider) reStageModified(ctx context.Context, tx ports.DBExec, item domain.Item, id, approverID string, editedPayload json.RawMessage) error {
	if err := d.Repo.Transition(ctx, tx, id, domain.StatusModified, approverID); err != nil {
		return fmt.Errorf("decider modify restage transition: %w", err)
	}

	authRule := "mcp.modify.restaged_yellow"
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       item.WorkspaceID,
		ActorType:         actorTypeHuman,
		ActorID:           approverID,
		Action:            actionModify,
		EntityType:        entityApproval,
		EntityID:          &id,
		Before:            map[string]any{"original_payload": string(item.Payload)},
		After:             map[string]any{"edited_payload": string(editedPayload), decisionKey: decisionModified, "restaged": true},
		AuthorizationRule: &authRule,
	}); err != nil {
		return fmt.Errorf("decider modify restage audit: %w", err)
	}

	newID, err := d.Repo.Create(ctx, tx, domain.Item{
		WorkspaceID: item.WorkspaceID,
		ActionType:  item.ActionType,
		Payload:     editedPayload,
		Status:      domain.StatusPending,
		RequestedBy: item.RequestedBy,
	})
	if err != nil {
		return fmt.Errorf("decider modify restage create: %w", err)
	}

	if d.Emitter != nil {
		p, _ := json.Marshal(map[string]string{decisionKey: decisionModified, itemIDKey: id, "restaged_as": newID})
		if err := d.Emitter.Emit(ctx, tx, topicApprovalDecided, item.WorkspaceID, id, p); err != nil {
			return fmt.Errorf("decider modify restage emit: %w", err)
		}
	}
	return nil
}

// execAction dispatches the datasource write for the item using the provided payload.
func (d Decider) execAction(ctx context.Context, item domain.Item, payload json.RawMessage) error {
	switch item.ActionType {
	case "update_record":
		in, err := parseUpdateInput(payload)
		if err != nil {
			return err
		}
		if _, err := d.Datasource.Update(ctx, in); err != nil {
			return fmt.Errorf("decider execAction update: %w", err)
		}
	case "create_record":
		in, err := parseCreateInput(payload)
		if err != nil {
			return err
		}
		if _, err := d.Datasource.Create(ctx, in); err != nil {
			return fmt.Errorf("decider execAction create: %w", err)
		}
	default:
		// send_email, archive_record, etc. — delegated to tool handlers.
	}
	return nil
}

// parseUpdateInput extracts a datasource.UpdateInput from a JSON payload.
func parseUpdateInput(payload json.RawMessage) (datasource.UpdateInput, error) {
	var data struct {
		Kind       string         `json:"kind"`
		ID         string         `json:"id"`
		Fields     map[string]any `json:"fields"`
		Source     string         `json:"source"`
		CapturedBy string         `json:"captured_by"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return datasource.UpdateInput{}, fmt.Errorf("parseUpdateInput: %w", err)
	}
	return datasource.UpdateInput{
		Type:       datasource.EntityType(data.Kind),
		ID:         data.ID,
		Patch:      data.Fields,
		Source:     data.Source,
		CapturedBy: data.CapturedBy,
	}, nil
}

// parseCreateInput extracts a datasource.CreateInput from a JSON payload.
func parseCreateInput(payload json.RawMessage) (datasource.CreateInput, error) {
	var data struct {
		Kind       string         `json:"kind"`
		Fields     map[string]any `json:"fields"`
		Source     string         `json:"source"`
		CapturedBy string         `json:"captured_by"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return datasource.CreateInput{}, fmt.Errorf("parseCreateInput: %w", err)
	}
	return datasource.CreateInput{
		Type:       datasource.EntityType(data.Kind),
		Fields:     data.Fields,
		Source:     data.Source,
		CapturedBy: data.CapturedBy,
	}, nil
}
