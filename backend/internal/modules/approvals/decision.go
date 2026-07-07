package crmapprovals

import (
	"context"
	"encoding/json"
	"fmt"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// Topic constants for approval lifecycle events. This package is the canonical
// topic source for the approval surface (crm-core is off-limits per ADR-0014).
const (
	TopicApprovalRequested = "approval.requested"
	TopicApprovalDecided   = "approval.decided"
)

// EventEmitter is a narrow seam for writing outbox events inside an open tx.
// The concrete implementation (writing to event_outbox) lives in cmd/server.
type EventEmitter interface {
	Emit(ctx context.Context, tx DBExec, topic, workspaceID, entityID string, payload json.RawMessage) error
}

// AdmitFunc is an injection boundary for re-admission on modify. It validates
// that the edited payload is within scope before the datasource write proceeds.
type AdmitFunc func(ctx context.Context, approverID string, actionType string, payload json.RawMessage) error

// Decider executes approval decisions (approve / reject / modify).
type Decider struct {
	Repo       Repository
	Datasource datasource.Provider
	Emitter    EventEmitter
	// ResolveYellow reports whether (action_type, payload) re-resolves to the 🟡
	// tier. Modify consults it on the EDITED payload: an edit that pushes a dynamic
	// action into 🟡 (e.g. advance_deal to_status=won) must not execute under the
	// original approval — it re-stages for its own approval cycle. crm-approvals
	// can't import mcp (ADR-0014), so the composition root injects this resolver.
	// nil means "tier is fixed for this action_type" (no re-resolution).
	ResolveYellow func(actionType string, payload json.RawMessage) bool
}

// Approve claims a pending approval_item and executes the underlying datasource action
// exactly-once.
//
// Ordering (the exactly-once invariant): the Datasource target may be an EXTERNAL incumbent
// that cannot enlist in this Postgres tx, so "same tx as exec" is impossible — a
// committed-then-crashed exec can't be rolled back. We therefore CLAIM the item
// FIRST and use the transition as the idempotency guard:
//
//  1. Transition pending→approved. Repo.Transition is a conditional
//     `UPDATE … WHERE status='pending'` gated on RowsAffected: exactly ONE caller
//     can win it. Two concurrent approvers both read pending at the Get above, but
//     only one's UPDATE affects a row — the loser gets RowsAffected==0 and errors
//     out BEFORE exec, so the action fires once. (The conditional UPDATE is the
//     row-lock; no separate FOR UPDATE is needed.)
//  2. execAction. Reached only by the winner. A crash AFTER exec leaves the item in
//     the terminal `approved` state — NOT re-fireable pending — so a re-approve
//     finds it non-pending and refuses. A crash BEFORE exec rolls the transition
//     back with the tx (the claim is released) and the item is safely re-approvable.
//
// The cost of this ordering: a crash strictly between commit-of-transition and exec
// would leave the item approved-but-unexecuted (a stuck approval), which is
// recoverable/visible — strictly safer than the previous ordering's silent
// double-fire of an irreversible action.
func (d Decider) Approve(ctx context.Context, tx DBExec, id, approverID string) error {
	item, err := d.Repo.Get(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("decider approve get: %w", err)
	}
	if item.Status != StatusPending {
		return fmt.Errorf("decider approve: item %s is not pending (status=%s)", id, item.Status)
	}

	if err := database.SetWorkspaceScope(ctx, tx, item.WorkspaceID); err != nil {
		return fmt.Errorf("decider approve guc: %w", err)
	}

	// 1. Claim the item: only one concurrent approver wins this conditional UPDATE.
	if err := d.Repo.Transition(ctx, tx, id, StatusApproved, approverID); err != nil {
		return fmt.Errorf("decider approve transition: %w", err)
	}

	// 2. Winner-only: execute the irreversible action after the claim is held.
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
		// fixed string map: marshal cannot fail.
		p, _ := json.Marshal(map[string]string{decisionKey: "approved", itemIDKey: id})
		if err := d.Emitter.Emit(ctx, tx, TopicApprovalDecided, item.WorkspaceID, id, p); err != nil {
			return fmt.Errorf("decider approve emit: %w", err)
		}
	}

	return nil
}

// Reject transitions a pending approval_item to rejected. No datasource side-effect.
func (d Decider) Reject(ctx context.Context, tx DBExec, id, approverID, reason string) error {
	item, err := d.Repo.Get(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("decider reject get: %w", err)
	}
	if item.Status != StatusPending {
		return fmt.Errorf("decider reject: item %s is not pending (status=%s)", id, item.Status)
	}

	if err := database.SetWorkspaceScope(ctx, tx, item.WorkspaceID); err != nil {
		return fmt.Errorf("decider reject guc: %w", err)
	}

	if err := d.Repo.Transition(ctx, tx, id, StatusRejected, approverID); err != nil {
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
		// fixed string map: marshal cannot fail.
		p, _ := json.Marshal(map[string]string{decisionKey: "rejected", itemIDKey: id})
		if err := d.Emitter.Emit(ctx, tx, TopicApprovalDecided, item.WorkspaceID, id, p); err != nil {
			return fmt.Errorf("decider reject emit: %w", err)
		}
	}

	return nil
}

// Modify transitions a pending approval_item to modified, runs the gate, then
// executes the datasource action with editedPayload. Writes two audit rows: one for
// the human delta, one for the original proposal reference.
func (d Decider) Modify(ctx context.Context, tx DBExec, id, approverID string, editedPayload json.RawMessage, gate AdmitFunc) error {
	item, err := d.Repo.Get(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("decider modify get: %w", err)
	}
	if item.Status != StatusPending {
		return fmt.Errorf("decider modify: item %s is not pending (status=%s)", id, item.Status)
	}

	// Gate check before any writes.
	if err := gate(ctx, approverID, item.ActionType, editedPayload); err != nil {
		return fmt.Errorf("decider modify gate: %w", err)
	}

	if err := database.SetWorkspaceScope(ctx, tx, item.WorkspaceID); err != nil {
		return fmt.Errorf("decider modify guc: %w", err)
	}

	// Re-resolve the tier on the EDITED payload before executing. A modify can push a
	// dynamic action into 🟡 (e.g. editing advance_deal to_status→won), and a 🟡 effect
	// must never auto-commit on the original approval. When the edit lands 🟡 we
	// re-stage it: the current item transitions to `modified` (this approval is spent)
	// and a fresh pending item carrying the edited payload enters its own approval
	// cycle. The datasource write does NOT fire here.
	if d.ResolveYellow != nil && d.ResolveYellow(item.ActionType, editedPayload) {
		return d.reStageModified(ctx, tx, item, id, approverID, editedPayload)
	}

	// Same exactly-once ordering as Approve: CLAIM the item (conditional transition,
	// only one caller wins) BEFORE executing the irreversible edited action, so a
	// concurrent decide can't double-fire and a post-exec crash leaves a terminal
	// `modified` state rather than a re-fireable pending one.
	if err := d.Repo.Transition(ctx, tx, id, StatusModified, approverID); err != nil {
		return fmt.Errorf("decider modify transition: %w", err)
	}

	if err := d.execAction(ctx, item, editedPayload); err != nil {
		return fmt.Errorf("decider modify exec: %w", err)
	}

	// First audit row: human delta (the edited payload).
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

	// Second audit row: original proposal reference.
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

// reStageModified handles a modify whose edited payload re-resolves to 🟡: the
// edited effect needs its own approval, so we spend the current approval (transition
// to `modified`, audited) and stage a fresh pending item with the edited payload.
// No datasource write occurs — the re-staged item must be approved on its own.
func (d Decider) reStageModified(ctx context.Context, tx DBExec, item Item, id, approverID string, editedPayload json.RawMessage) error {
	if err := d.Repo.Transition(ctx, tx, id, StatusModified, approverID); err != nil {
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

	// Stage the edited payload as a brand-new pending item for its own approval cycle.
	newID, err := d.Repo.Create(ctx, tx, Item{
		WorkspaceID: item.WorkspaceID,
		ActionType:  item.ActionType,
		Payload:     editedPayload,
		Status:      StatusPending,
		RequestedBy: item.RequestedBy,
	})
	if err != nil {
		return fmt.Errorf("decider modify restage create: %w", err)
	}

	if d.Emitter != nil {
		// fixed string map: marshal cannot fail.
		p, _ := json.Marshal(map[string]string{decisionKey: decisionModified, itemIDKey: id, "restaged_as": newID})
		if err := d.Emitter.Emit(ctx, tx, TopicApprovalDecided, item.WorkspaceID, id, p); err != nil {
			return fmt.Errorf("decider modify restage emit: %w", err)
		}
	}
	return nil
}

// execAction dispatches the datasource write for the item using the provided payload.
// For send_email / archive_record and other non-datasource action types this is a no-op.
func (d Decider) execAction(ctx context.Context, item Item, payload json.RawMessage) error {
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
