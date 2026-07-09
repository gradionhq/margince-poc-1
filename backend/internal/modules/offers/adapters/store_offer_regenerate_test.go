//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func TestOfferStore_Regenerate_GroundedAndUngroundedSignals(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)
	created := newDraftOffer(t, db, wsID, dealID)
	forceOfferSent(t, db, created.ID)

	price := int64(500)
	signals := []domain.OfferLineSignal{
		{Description: "Consulting", Quantity: 2, UnitPriceMinor: &price, Snippet: "2 days consulting requested", SourceID: "activity-1"},
		{Description: "Onboarding (no price)", Quantity: 1, Snippet: "onboarding mentioned, no price discussed", SourceID: "activity-2"},
		{Description: "Ungrounded", Quantity: 1, Snippet: "", SourceID: ""},
	}

	regenerated, err := s.Regenerate(context.Background(), created.ID, wsID, signals)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if regenerated.Status != domain.OfferStatusDraft || regenerated.Revision != created.Revision+1 {
		t.Fatalf("expected new draft revision=%d, got status=%s revision=%d", created.Revision+1, regenerated.Status, regenerated.Revision)
	}
	if !regenerated.AIGenerated || regenerated.AIDisclosure == nil || *regenerated.AIDisclosure == "" {
		t.Fatalf("expected ai_generated=true + non-empty disclosure, got %+v", regenerated)
	}
	if regenerated.DiffFromPrevious == nil || len(regenerated.DiffFromPrevious.Added) != 2 {
		t.Fatalf("expected exactly 2 added lines in the diff (the ungrounded signal is dropped), got %+v", regenerated.DiffFromPrevious)
	}

	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	lines, err := lineStore.List(context.Background(), regenerated.ID, wsID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 persisted lines (evidence-or-omit dropped the 3rd), got %d: %+v", len(lines), lines)
	}
	for _, li := range lines {
		if li.Evidence == nil || li.Evidence.Snippet == "" || li.Evidence.SourceID == "" {
			t.Fatalf("expected every persisted AI line to carry non-empty evidence, got %+v", li)
		}
		switch li.Description {
		case "Consulting":
			if li.UnitPriceMinor != 500 || !li.PriceGrounded {
				t.Fatalf("expected the priced signal's line to carry unit_price_minor=500, price_grounded=true, got %+v", li)
			}
		case "Onboarding (no price)":
			if li.UnitPriceMinor != 0 || li.PriceGrounded {
				t.Fatalf("expected the unpriced signal's line to carry unit_price_minor=0, price_grounded=false, got %+v", li)
			}
		default:
			t.Fatalf("unexpected line persisted: %+v", li)
		}
	}

	// Consulting: qty=2 * unit_price_minor=500 = 1000; Onboarding: qty=1 *
	// unit_price_minor=0 = 0. Neither signal carries a product_id, so
	// tax_rate defaults to 0 on both lines, so net==gross==1000. This proves
	// recomputeOfferTotals/computeLineTotals are exercised on a nonzero path.
	const wantTotalMinor = 1000
	if regenerated.NetMinor != wantTotalMinor || regenerated.GrossMinor != wantTotalMinor {
		t.Fatalf("expected net_minor=gross_minor=%d from the two grounded lines, got net=%d gross=%d", wantTotalMinor, regenerated.NetMinor, regenerated.GrossMinor)
	}

	prior, err := s.Get(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("get prior: %v", err)
	}
	if prior.Status != domain.OfferStatusSuperseded {
		t.Fatalf("expected prior status=superseded, got %s", prior.Status)
	}

	var eventCount, auditCount, dealEventCount int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='offer.superseded' AND entity_id=$1::uuid`, created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected exactly one offer.superseded event keyed on the prior offer id, got %d", eventCount)
	}
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid`, regenerated.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected exactly one audit_log entry for the new offer, got %d", auditCount)
	}
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic LIKE 'deal.%' AND entity_id=$1::uuid`, dealID).Scan(&dealEventCount); err != nil {
		t.Fatalf("count deal events: %v", err)
	}
	if dealEventCount != 0 {
		t.Fatalf("expected zero deal.* events from a regenerate call, got %d", dealEventCount)
	}
}

func TestOfferStore_Regenerate_NoContextFixture_EmptyDraft(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)
	created := newDraftOffer(t, db, wsID, dealID)
	forceOfferSent(t, db, created.ID)

	regenerated, err := s.Regenerate(context.Background(), created.ID, wsID, nil)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	lines, err := lineStore.List(context.Background(), regenerated.ID, wsID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected an honest empty draft for a no-context fixture, got %d lines", len(lines))
	}
	if regenerated.NetMinor != 0 || regenerated.GrossMinor != 0 {
		t.Fatalf("expected zero totals for a lineless regenerate, got net=%d gross=%d", regenerated.NetMinor, regenerated.GrossMinor)
	}
}

// TestOfferStore_Regenerate_NotSent_Rejected covers both non-sent states a
// regenerate call must reject: a still-draft offer, and an already-
// superseded one — regenerate's precondition is requireSent (crm.yaml's
// regenerateOffer: "regenerate a sent offer"; OFFER-AC-10d), not
// requireDraft (draft is a *destination*, never a precondition, for this
// verb).
func TestOfferStore_Regenerate_NotSent_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	s := adapters.NewOfferStore(db)

	// newDraftOffer derives a deterministic offer_number from its
	// (wsID, dealID) pair, so the two fixtures below each need their own
	// seedOfferWorkspace call to avoid an offer_number+revision collision.
	wsID1, dealID1 := seedOfferWorkspace(t, db)
	stillDraft := newDraftOffer(t, db, wsID1, dealID1)
	if _, err := s.Regenerate(context.Background(), stillDraft.ID, wsID1, nil); !errors.Is(err, adapters.ErrOfferNotSent) {
		t.Fatalf("expected ErrOfferNotSent for a still-draft offer, got %v", err)
	}

	wsID2, dealID2 := seedOfferWorkspace(t, db)
	superseded := newDraftOffer(t, db, wsID2, dealID2)
	if _, err := db.Exec(`UPDATE offer SET status='superseded' WHERE id=$1::uuid`, superseded.ID); err != nil {
		t.Fatalf("force status=superseded: %v", err)
	}
	if _, err := s.Regenerate(context.Background(), superseded.ID, wsID2, nil); !errors.Is(err, adapters.ErrOfferNotSent) {
		t.Fatalf("expected ErrOfferNotSent, got %v", err)
	}
}

// forceOfferSent flips id's status straight to sent for tests that need a
// regenerate-eligible fixture without exercising the full Send flow.
func forceOfferSent(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE offer SET status='sent' WHERE id=$1::uuid`, id); err != nil {
		t.Fatalf("force status=sent: %v", err)
	}
}

func TestOfferStore_Regenerate_RateCardFallback_WhenSignalHasNoPrice(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	products := adapters.NewProductStore(db)

	p := domain.NewProduct("Consulting Day", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	p.UnitPriceMinor = 75000
	p.Currency = "EUR"
	createdProduct, err := products.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}

	s := adapters.NewOfferStore(db)
	created := newDraftOffer(t, db, wsID, dealID)
	forceOfferSent(t, db, created.ID)
	signals := []domain.OfferLineSignal{
		{Description: "Consulting Day", Quantity: 1, ProductID: &createdProduct.ID, Snippet: "wants a consulting day", SourceID: "activity-1"},
	}

	regenerated, err := s.Regenerate(context.Background(), created.ID, wsID, signals)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	lineStore := adapters.NewOfferLineItemStore(db, products)
	lines, err := lineStore.List(context.Background(), regenerated.ID, wsID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	if len(lines) != 1 || lines[0].UnitPriceMinor != 75000 || !lines[0].PriceGrounded {
		t.Fatalf("expected the rate-card price to fill the ungrounded-price signal (price_grounded=true), got %+v", lines)
	}
}

func TestOfferStore_Regenerate_OnlyGroundedSignalsPersist(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)
	created := newDraftOffer(t, db, wsID, dealID)
	forceOfferSent(t, db, created.ID)

	good := domain.OfferLineSignal{Description: "Consulting", Quantity: 1, Snippet: "consulting mention", SourceID: "activity-1"}
	bad := domain.OfferLineSignal{Description: "Ignore me", Quantity: 1}
	regenerated, err := s.Regenerate(context.Background(), created.ID, wsID, []domain.OfferLineSignal{bad, good})
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	lines, err := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db)).List(context.Background(), regenerated.ID, wsID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	if len(lines) != 1 || lines[0].Description != "Consulting" {
		t.Fatalf("expected only the grounded signal to persist, got %+v", lines)
	}
}

func TestOfferStore_Regenerate_ChangeDiff_TracksCoreFields(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	s := adapters.NewOfferStore(db)
	created := newDraftOffer(t, db, wsID, dealID)
	forceOfferSent(t, db, created.ID)

	first := int64(100)
	second := int64(200)
	base := []domain.OfferLineSignal{
		{Description: "Consulting", Quantity: 1, UnitPriceMinor: &first, Snippet: "consulting", SourceID: "activity-1"},
	}
	firstRegenerated, err := s.Regenerate(context.Background(), created.ID, wsID, base)
	if err != nil {
		t.Fatalf("first regenerate: %v", err)
	}
	forceOfferSent(t, db, firstRegenerated.ID)

	updated := []domain.OfferLineSignal{
		{Description: "Consulting", Quantity: 2, UnitPriceMinor: &second, Snippet: "consulting updated", SourceID: "activity-2"},
	}
	regenerated, err := s.Regenerate(context.Background(), firstRegenerated.ID, wsID, updated)
	if err != nil {
		t.Fatalf("second regenerate: %v", err)
	}
	if regenerated.DiffFromPrevious == nil || len(regenerated.DiffFromPrevious.Changed) != 1 {
		t.Fatalf("expected one changed line in the diff, got %+v", regenerated.DiffFromPrevious)
	}
	if regenerated.DiffFromPrevious.Changed[0].Before.Quantity != 1 || regenerated.DiffFromPrevious.Changed[0].After.Quantity != 2 {
		t.Fatalf("expected quantity diff to be tracked, got %+v", regenerated.DiffFromPrevious.Changed[0])
	}
}
