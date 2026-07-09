package app

import (
	"context"
	"encoding/json"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// ActivityLogger is the narrow send seam this ticket owns. A recovery approval
// can become a real activity only through this interface; the module does not
// wire a datasource.Provider seam here.
type ActivityLogger interface {
	LogFollowUp(ctx context.Context, tx crmapprovals.DBExec, workspaceID, dealID, subject, body, source, capturedBy string) (activityID string, err error)
}

type stalledRecoveryDraft struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type stalledRecoveryEffectPayload struct {
	Reason             string                `json:"reason"`
	EvidenceActivityID string                `json:"evidence_activity_id"`
	DealID             string                `json:"deal_id"`
	WorkspaceID        string                `json:"workspace_id"`
	Draft              *stalledRecoveryDraft `json:"draft"`
}

// StalledRecoveryEffector turns an approved stalled_recovery payload into a
// logged follow-up. The returned string is the created activity ID, which the
// caller records as a locate/correct rollback handle rather than a real undo.
type StalledRecoveryEffector struct {
	Logger ActivityLogger
}

func (e StalledRecoveryEffector) Apply(ctx context.Context, tx crmapprovals.DBExec, _ string, payload json.RawMessage) (string, error) {
	var effect stalledRecoveryEffectPayload
	if err := json.Unmarshal(payload, &effect); err != nil {
		return "", nil
	}
	if effect.Draft == nil || effect.Draft.Subject == "" || effect.Draft.Body == "" {
		return "", nil
	}
	if e.Logger == nil {
		return "", nil
	}
	return e.Logger.LogFollowUp(ctx, tx, effect.WorkspaceID, effect.DealID, effect.Draft.Subject, effect.Draft.Body, "overnight.stalled_recovery", ActorOvernight)
}
