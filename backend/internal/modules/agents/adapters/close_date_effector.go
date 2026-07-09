package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
)

// closeDateActionTypes are the only action types this effector handles — the
// two green writes (applied within the same pass) and the two yellow writes
// (applied later, at human-decision time, via the SAME effector keyed by
// action type, exactly as executor.go's HandleDecided already does for any
// approved item).
var closeDateActionTypes = map[string]struct{}{
	"close-date-auto-apply":       {},
	"close-date-provisional-set":  {},
	"close-date-confirm-request":  {},
	"close-date-downgrade-review": {},
}

// closeDateEffect is the payload every close-date-* proposal/effect carries.
type closeDateEffect struct {
	DealID         string  `json:"deal_id"`
	WorkspaceID    string  `json:"workspace_id"`
	IfMatch        int64   `json:"if_match"`
	NewCloseDate   string  `json:"new_close_date"`
	PriorCloseDate *string `json:"prior_close_date"`
	PriorVersion   int64   `json:"prior_version"`
	EventTopic     string  `json:"event_topic,omitempty"`
}

// CloseDateEffector implements ports.Effector for every close-date-* action
// type, writing through deals.DealStore.Update's existing If-Match path —
// never a private write path (the ticket's own Context note).
type CloseDateEffector struct {
	dealStore *deals.DealStore
}

// NewCloseDateEffector returns a CloseDateEffector backed by dealStore.
func NewCloseDateEffector(dealStore *deals.DealStore) *CloseDateEffector {
	return &CloseDateEffector{dealStore: dealStore}
}

var _ ports.Effector = (*CloseDateEffector)(nil)

// Apply unmarshals the close-date effect payload and writes
// expected_close_date via DealStore.Update (If-Match), returning a rollback
// handle sufficient to reverse via another plain Update call (OVN-GAP-1 — no
// dedicated rollback op; reversal is a human/operator action, not automated
// here). The tx parameter is unused: DealStore.Update always opens its own
// transaction (Pre-implementation Finding 2) — accepted per the ticket's own
// design, not a private write path substitute.
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
		"deal_id":          eff.DealID,
		"prior_close_date": eff.PriorCloseDate,
		"prior_version":    eff.PriorVersion,
	})
	if err != nil {
		return "", fmt.Errorf("close date effector: rollback marshal: %w", err)
	}
	return string(rollback), nil
}
