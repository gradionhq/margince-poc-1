// Package app — see gate.go's package doc. This file adds the first real
// proposal producer over the overnight-agent spine: field-change signals
// (stage/next_step/amount -- not close_date, ONA-T02's separate
// deterministic producer over the same spine). A producer's only contract
// is ports.Produce's signature (see producer_reconciliation.go); it reads
// domain.AssembledView.Facts exactly as ONA-T01 defined them and never
// touches a shared spine file.
package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// EntityTypeFieldChangeSignal is the Fact.EntityType this producer
// recognizes. Fact.Detail is a JSON-encoded fieldChangeSignal (below) --
// this producer's own convention layered on Fact's plain string Detail,
// rather than widening the shared domain.Fact struct (avoids a merge
// collision with ONA-T02, built concurrently over the same shared types).
const EntityTypeFieldChangeSignal = "field_change_signal"

// allowedFieldChangeFields are the only field names this producer will
// ever propose a change for. "close_date" is deliberately absent --
// ONA-T02 owns close-date hygiene as its own deterministic producer.
var allowedFieldChangeFields = map[string]bool{
	"stage":     true,
	"next_step": true,
	"amount":    true,
}

// fieldChangeSignal is the Fact.Detail JSON shape ProduceFieldChanges
// parses: a proposed change to one field on one deal, plus the evidence
// and confidence a human needs to judge it. Value is left as raw JSON so
// both string fields (stage/next_step) and a numeric one (amount)
// round-trip without a second producer-specific type.
type fieldChangeSignal struct {
	DealID     string          `json:"deal_id"`
	Field      string          `json:"field"`
	Value      json.RawMessage `json:"value"`
	Evidence   string          `json:"evidence"`
	Confidence *float64        `json:"confidence"`
}

// fieldChangeEffect is Proposal.Effect's JSON shape -- shaped so a future
// effector could apply it directly; this ticket builds no such effector.
type fieldChangeEffect struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

// ProduceFieldChanges converts supported field-change facts into staged
// proposals. Invalid, incomplete, or irrelevant facts are silently
// skipped so the producer never guesses.
func ProduceFieldChanges(view domain.AssembledView) ([]domain.Proposal, error) {
	out := make([]domain.Proposal, 0, len(view.Facts))
	for _, fact := range view.Facts {
		if fact.EntityType != EntityTypeFieldChangeSignal {
			continue
		}
		var signal fieldChangeSignal
		if err := json.Unmarshal([]byte(fact.Detail), &signal); err != nil {
			continue
		}
		if signal.DealID == "" || signal.Field == "" || signal.Evidence == "" || signal.Confidence == nil {
			continue
		}
		if !allowedFieldChangeFields[signal.Field] {
			continue
		}
		if signal.Field == "close_date" {
			continue
		}
		effect, err := json.Marshal(fieldChangeEffect{Field: signal.Field, Value: signal.Value})
		if err != nil {
			continue
		}
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "field_change",
			TargetEntity: "deal:" + signal.DealID,
			Effect:       effect,
			Evidence:     signal.Evidence,
			Confidence:   signal.Confidence,
			Source:       fact.Source,
		})
	}
	return out, nil
}
