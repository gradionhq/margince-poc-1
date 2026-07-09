package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
)

var closeDateActionTypes = map[string]struct{}{
	"close-date-auto-apply":      {},
	"close-date-provisional-set": {},
	"close-date-confirm-request": {},
	"close-date-downgrade-review": {},
}

type closeDateEffect struct {
	DealID         string  `json:"deal_id"`
	WorkspaceID    string  `json:"workspace_id"`
	IfMatch        int64   `json:"if_match"`
	NewCloseDate   string  `json:"new_close_date"`
	PriorCloseDate *string `json:"prior_close_date"`
	PriorVersion   int64   `json:"prior_version"`
	EventTopic     string  `json:"event_topic,omitempty"`
}

type CloseDateEffector struct {
	dealStore *deals.DealStore
}

func NewCloseDateEffector(dealStore *deals.DealStore) *CloseDateEffector {
	return &CloseDateEffector{dealStore: dealStore}
}

var _ ports.Effector = (*CloseDateEffector)(nil)

func (e *CloseDateEffector) Apply(ctx context.Context, _ ports.DBExec, actionType string, payload json.RawMessage) (string, error) {
	if _, ok := closeDateActionTypes[actionType]; !ok {
		return "", fmt.Errorf("close date effector: unsupported action_type %q", actionType)
	}

	var eff closeDateEffect
	if err := json.Unmarshal(payload, &eff); err != nil {
		return "", fmt.Errorf("close date effector: unmarshal: %w", err)
	}

	if _, err := e.dealStore.Update(ctx, eff.DealID, eff.WorkspaceID, map[string]any{
		"expected_close_date": eff.NewCloseDate,
	}, eff.IfMatch); err != nil {
		return "", fmt.Errorf("close date effector: update: %w", err)
	}

	rollback, err := json.Marshal(map[string]any{
		"deal_id":         eff.DealID,
		"prior_close_date": eff.PriorCloseDate,
		"prior_version":   eff.PriorVersion,
	})
	if err != nil {
		return "", fmt.Errorf("close date effector: rollback marshal: %w", err)
	}
	return string(rollback), nil
}
