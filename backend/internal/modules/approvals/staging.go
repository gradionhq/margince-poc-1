package crmapprovals

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// DefaultApprovalTTL is the expiry window applied to a staged approval_item when
// StageInput.TTL is zero and pgRepository.Create receives a nil ExpiresAt.
const DefaultApprovalTTL = 72 * time.Hour

// StageInput carries the parameters for a commit-block staging call.
type StageInput struct {
	WorkspaceID        string
	ActionType         string
	RequestedBy        string
	Payload            json.RawMessage
	DryRunPreview      json.RawMessage
	TrustTiers         json.RawMessage
	ContentEgressFlags json.RawMessage
	TTL                time.Duration // default 72h when zero
}

// Stage is the commit-block: it writes a pending approval_item + one audit_log
// row (action=capture) in the caller's tx and returns ErrRequiresApproval.
// No primary-table write occurs — Stage only touches approval_item + audit_log.
//
// B6: Stage sets the app.workspace_id GUC for the caller's tx so both FORCE-RLS
// INSERTs pass (same pattern as crmaudit.Write and crmgdpr.Record).
func Stage(ctx context.Context, tx DBExec, repo Repository, in StageInput) (string, error) {
	if in.WorkspaceID == "" {
		return "", fmt.Errorf("crmapprovals stage: empty workspace_id")
	}
	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.workspace_id', $1, true)`, in.WorkspaceID); err != nil {
		return "", fmt.Errorf("crmapprovals stage guc: %w", err)
	}

	ttl := in.TTL
	if ttl == 0 {
		ttl = DefaultApprovalTTL
	}
	expiresAt := time.Now().Add(ttl)

	itemID, err := repo.Create(ctx, tx, Item{
		WorkspaceID:        in.WorkspaceID,
		ActionType:         in.ActionType,
		Payload:            in.Payload,
		DryRunPreview:      in.DryRunPreview,
		TrustTiers:         in.TrustTiers,
		ContentEgressFlags: in.ContentEgressFlags,
		Status:             StatusPending,
		RequestedBy:        in.RequestedBy,
		ExpiresAt:          &expiresAt,
	})
	if err != nil {
		return "", fmt.Errorf("crmapprovals stage create: %w", err)
	}

	authRule := "mcp.pending_approval"
	_, err = crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       in.WorkspaceID,
		ActorType:         "agent",
		ActorID:           in.RequestedBy,
		Action:            "capture",
		EntityType:        entityApproval,
		EntityID:          &itemID,
		After:             map[string]any{"state": "pending_approval"},
		AuthorizationRule: &authRule,
	})
	if err != nil {
		return "", fmt.Errorf("crmapprovals stage audit: %w", err)
	}

	return itemID, errs.ErrRequiresApproval
}
