package app

import (
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
)

// ReconciliationProduce is this ticket's ports.Produce implementation:
// the first real proposal producer over the overnight-agent spine ONA-T01
// shipped. It concatenates the three signal-specific producers' output,
// order-preserving (field changes, then new contacts, then follow-ups --
// BuildBatch, app/batch.go, untouched by this ticket, re-sorts by
// group/confidence downstream anyway, so this order is deterministic-test
// convenience only, never a contract).
//
// None of the three producers ever returns a non-nil error in practice --
// a malformed or incomplete fact is silently skipped, never an error --
// so ReconciliationProduce always returns a nil error too. A degraded run
// (RunPass/BuildBatch's producerErr path) is reserved for a future
// producer that can genuinely partial-fail, e.g. a live ContextAssembler
// outage; not applicable to this fixture-driven producer, which only
// ever reads the view.Facts it was already handed.
func ReconciliationProduce(view domain.AssembledView) ([]domain.Proposal, error) {
	var out []domain.Proposal

	fieldChanges, _ := ProduceFieldChanges(view)
	out = append(out, fieldChanges...)

	newContacts, _ := ProduceNewContacts(view)
	out = append(out, newContacts...)

	followUps, _ := ProduceFollowUps(view)
	out = append(out, followUps...)

	return out, nil
}

// var _ proves ReconciliationProduce satisfies ports.Produce's exact
// shape -- the ONLY seam this ticket plugs into the existing pipeline
// (PassInput.Produce).
var _ ports.Produce = ReconciliationProduce
