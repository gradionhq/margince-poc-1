package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// EntityTypeFollowUpSignal is the Fact.EntityType this producer
// recognizes -- an already-drafted follow-up (subject + body) an upstream
// signal supplied. This producer never generates text itself: per the
// subsystem doc's "with no model available... generative drafts are
// omitted rather than guessed," a signal with an empty body is skipped,
// not invented.
const EntityTypeFollowUpSignal = "followup_signal"

// followUpSignal is the Fact.Detail JSON shape ProduceFollowUps parses.
// Target is the related deal/contact, already shaped "deal:<id>" or
// "contact:<id>".
type followUpSignal struct {
	Target     string   `json:"target"`
	Recipient  string   `json:"recipient"`
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
	Evidence   string   `json:"evidence"`
	Confidence *float64 `json:"confidence"`
}

// followUpEffect is Proposal.Effect's JSON shape.
type followUpEffect struct {
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

// ProduceFollowUps scans view.Facts for EntityTypeFollowUpSignal facts and
// emits one draft_followup Proposal per signal carrying a non-empty draft
// body and every no-guess-gate field. ActionType "draft_followup" is
// registered in neither d4FloorActionTypes nor greenActionTypes
// (app/tier.go, untouched by this ticket), so it always default-denies to
// the 🟡 floor -- staged, never sent; the send-with-provenance path is a
// future producer's job (ONA-T05), not built here.
func ProduceFollowUps(view domain.AssembledView) ([]domain.Proposal, error) {
	out := make([]domain.Proposal, 0, len(view.Facts))
	for _, fact := range view.Facts {
		if fact.EntityType != EntityTypeFollowUpSignal {
			continue
		}
		var sig followUpSignal
		if err := json.Unmarshal([]byte(fact.Detail), &sig); err != nil {
			continue
		}
		if sig.Body == "" {
			continue
		}
		if sig.Target == "" || sig.Evidence == "" || sig.Confidence == nil {
			continue
		}
		effect, err := json.Marshal(followUpEffect{Recipient: sig.Recipient, Subject: sig.Subject, Body: sig.Body})
		if err != nil {
			continue
		}
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "draft_followup",
			TargetEntity: sig.Target,
			Effect:       effect,
			Evidence:     sig.Evidence,
			Confidence:   sig.Confidence,
			Source:       fact.Source,
		})
	}
	return out, nil
}
