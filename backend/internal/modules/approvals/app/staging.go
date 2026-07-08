// Package app contains the approvals module's application-service (use-case) layer.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
	"github.com/gradionhq/margince/backend/internal/modules/approvals/ports"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
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
func Stage(ctx context.Context, tx ports.DBExec, repo ports.Repository, in StageInput) (string, error) {
	if in.WorkspaceID == "" {
		return "", fmt.Errorf("crmapprovals stage: empty workspace_id")
	}
	if err := database.SetWorkspaceScope(ctx, tx, in.WorkspaceID); err != nil {
		return "", fmt.Errorf("crmapprovals stage guc: %w", err)
	}

	ttl := in.TTL
	if ttl == 0 {
		ttl = DefaultApprovalTTL
	}
	expiresAt := time.Now().Add(ttl)

	itemID, err := repo.Create(ctx, tx, domain.Item{
		WorkspaceID:        in.WorkspaceID,
		ActionType:         in.ActionType,
		Payload:            in.Payload,
		DryRunPreview:      in.DryRunPreview,
		TrustTiers:         in.TrustTiers,
		ContentEgressFlags: in.ContentEgressFlags,
		Status:             domain.StatusPending,
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
		EntityType:        "approval_item",
		EntityID:          &itemID,
		After:             map[string]any{"state": "pending_approval"},
		AuthorizationRule: &authRule,
	})
	if err != nil {
		return "", fmt.Errorf("crmapprovals stage audit: %w", err)
	}

	return itemID, errs.ErrRequiresApproval
}
