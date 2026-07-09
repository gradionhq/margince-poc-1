package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// EntityTypeNewContactSignal is the Fact.EntityType this producer
// recognizes -- a newly-detected contact worth creating, surfaced from
// captured activity (e.g. an email signature block, a name+number
// mentioned on a call). Fact.Detail is a JSON-encoded newContactSignal.
const EntityTypeNewContactSignal = "new_contact_signal"

// newContactSignal is the Fact.Detail JSON shape ProduceNewContacts
// parses. RelatesTo is the deal/org this contact surfaced in relation to,
// already shaped "deal:<id>" or "org:<id>" -- this producer trusts the
// upstream signal for that shape; it does not resolve or validate it.
type newContactSignal struct {
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	RelatesTo  string   `json:"relates_to"`
	Evidence   string   `json:"evidence"`
	Confidence *float64 `json:"confidence"`
}

// newContactEffect is Proposal.Effect's JSON shape.
type newContactEffect struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Relation string `json:"relation"`
}

// ProduceNewContacts scans view.Facts for EntityTypeNewContactSignal
// facts and emits one create_contact Proposal per signal carrying a name,
// at least one of email/phone, a target relation, and every no-guess-gate
// field -- a signal with no name, or with neither email nor phone, is not
// enough to act on and is skipped, not proposed.
func ProduceNewContacts(view domain.AssembledView) ([]domain.Proposal, error) {
	out := make([]domain.Proposal, 0, len(view.Facts))
	for _, fact := range view.Facts {
		if fact.EntityType != EntityTypeNewContactSignal {
			continue
		}
		var sig newContactSignal
		if err := json.Unmarshal([]byte(fact.Detail), &sig); err != nil {
			continue
		}
		if sig.Name == "" || (sig.Email == "" && sig.Phone == "") {
			continue
		}
		if sig.RelatesTo == "" || sig.Evidence == "" || sig.Confidence == nil {
			continue
		}
		effect, err := json.Marshal(newContactEffect{Name: sig.Name, Email: sig.Email, Phone: sig.Phone, Relation: sig.RelatesTo})
		if err != nil {
			continue
		}
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "create_contact",
			TargetEntity: sig.RelatesTo,
			Effect:       effect,
			Evidence:     sig.Evidence,
			Confidence:   sig.Confidence,
			Source:       fact.Source,
		})
	}
	return out, nil
}
