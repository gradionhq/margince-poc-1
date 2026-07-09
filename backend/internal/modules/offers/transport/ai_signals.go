package transport

import (
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/ports/retrieval"
)

const offerLineSignalsKey = "offer_line_signals"

func decodeOfferLineSignals(ctx retrieval.Context) []domain.OfferLineSignal {
	if ctx.Raw == nil {
		return nil
	}
	signals, ok := ctx.Raw[offerLineSignalsKey].([]domain.OfferLineSignal)
	if !ok {
		return nil
	}
	return signals
}
