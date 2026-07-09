//go:build integration

package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func f64Ptr(f float64) *float64 { return &f }
func i64Ptr(i int64) *int64     { return &i }

func newDraftOffer(t *testing.T, db *sql.DB, wsID, dealID string) domain.Offer {
	t.Helper()
	s := adapters.NewOfferStore(db)
	o := domain.NewOffer(dealID, "ANG-LI-"+wsID[:8]+dealID[:8], "EUR", provTestOffer())
	o.WorkspaceID = wsID
	created, err := s.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("seed draft offer: %v", err)
	}
	return created
}

func TestOfferLineItemStore_Create_ProductSnapshot(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	products := adapters.NewProductStore(db)
	lineStore := adapters.NewOfferLineItemStore(db, products)
	offer := newDraftOffer(t, db, wsID, dealID)

	p := domain.NewProduct("Consulting Day", provTestOffer())
	p.WorkspaceID = wsID
	p.UnitPriceMinor = 150000
	p.Currency = "EUR"
	dtr := 19.0
	p.DefaultTaxRate = &dtr
	createdProduct, err := products.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		ProductID: &createdProduct.ID, Description: "placeholder", Quantity: 2, UnitPriceMinor: i64Ptr(1),
		Source: "test", CapturedBy: "human:test",
	}
	created, err := lineStore.Create(context.Background(), li, nil)
	if err != nil {
		t.Fatalf("create line: %v", err)
	}
	if created.Description != "Consulting Day" || created.UnitPriceMinor == nil || *created.UnitPriceMinor != 150000 || created.TaxRate != 19.0 {
		var price any = nil
		if created.UnitPriceMinor != nil {
			price = *created.UnitPriceMinor
		}
		t.Fatalf("expected product snapshot copied onto the line, got description=%q unit_price_minor=%v tax_rate=%v",
			created.Description, price, created.TaxRate)
	}

	// Mutate the product's price after the line was written; the already-
	// written line must be unaffected (OFFER-AC-9b).
	if _, err := products.Update(context.Background(), createdProduct.ID, wsID, map[string]any{"unit_price_minor": float64(999999)}, 0); err != nil {
		t.Fatalf("mutate product price: %v", err)
	}
	reGot, err := lineStore.List(context.Background(), offer.ID, wsID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	for _, li := range reGot {
		if li.ID == created.ID && (li.UnitPriceMinor == nil || *li.UnitPriceMinor != 150000) {
			var price any = nil
			if li.UnitPriceMinor != nil {
				price = *li.UnitPriceMinor
			}
			t.Fatalf("expected line's snapshot price unchanged after product price mutation, got %v", price)
		}
	}
}

func TestOfferLineItemStore_Evidence_RoundTrips(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)
	evidence := &domain.Evidence{Snippet: "the customer asked for consulting", SourceID: "activity-1"}

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "Consulting", Quantity: 2, UnitPriceMinor: i64Ptr(150000),
		Evidence: evidence,
		Source:   "test", CapturedBy: "human:test",
	}
	created, err := lineStore.Create(context.Background(), li, nil)
	if err != nil {
		t.Fatalf("create line: %v", err)
	}
	if created.Evidence == nil || created.Evidence.Snippet != evidence.Snippet || created.Evidence.SourceID != evidence.SourceID {
		t.Fatalf("expected create to round-trip evidence, got %+v", created.Evidence)
	}

	updated, err := lineStore.Update(context.Background(), created.ID, offer.ID, wsID, map[string]any{"description": "Consulting (updated)"})
	if err != nil {
		t.Fatalf("update line: %v", err)
	}
	if updated.Evidence == nil || updated.Evidence.Snippet != evidence.Snippet || updated.Evidence.SourceID != evidence.SourceID {
		t.Fatalf("expected update to preserve evidence, got %+v", updated.Evidence)
	}

	lines, err := lineStore.List(context.Background(), offer.ID, wsID)
	if err != nil {
		t.Fatalf("list lines: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d", len(lines))
	}
	if lines[0].Evidence == nil || lines[0].Evidence.Snippet != evidence.Snippet || lines[0].Evidence.SourceID != evidence.SourceID {
		t.Fatalf("expected list to round-trip evidence, got %+v", lines[0].Evidence)
	}
}

// TestOfferLineItemStore_Reconciliation_RoundThenSum_DivergesFromSumThenRound
// is OFFER-AC-3's real assertion: two lines, each with a discount and a
// non-zero tax rate, chosen so round-then-sum and sum-then-round produce
// DIFFERENT totals — proving the store actually rounds per-line before
// summing, not merely that it sums then rounds once (which would silently
// pass a test whose numbers happen to agree under both strategies).
//
// Each line: quantity=1, unit_price_minor=201, discount_pct=50, tax_rate=50.
//
//	raw net = 1 * 201 * (1 - 0.50) = 100.5
//	round-then-sum:  line_net = round(100.5) = 101 (Go rounds .5 away from
//	  zero); line_tax = round(101 * 0.50) = round(50.5) = 51.
//	  Two lines -> offer net=202, tax=102, gross=304.
//	sum-then-round (the WRONG strategy this test guards against): sum the
//	  raw (unrounded) nets = 100.5+100.5 = 201.0 -> round once = 201; sum
//	  the raw taxes (100.5*0.5)+(100.5*0.5) = 100.5 -> round once = 101.
//	  201 != 202 and 101 != 102 — a genuine one-unit divergence in both
//	  net and tax, so a regression to sum-then-round fails this test.
func TestOfferLineItemStore_Reconciliation_RoundThenSum_DivergesFromSumThenRound(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)

	for pos := 1; pos <= 2; pos++ {
		li := domain.OfferLineItem{
			WorkspaceID: wsID, OfferID: offer.ID, Position: pos,
			Description: "Divergence line", Quantity: 1, UnitPriceMinor: i64Ptr(201), DiscountPct: 50,
			Source: "test", CapturedBy: "human:test",
		}
		if _, err := lineStore.Create(context.Background(), li, f64Ptr(50)); err != nil {
			t.Fatalf("create line %d: %v", pos, err)
		}
	}

	// Independently compute expected totals with the same round-then-sum
	// formula the store must use (OFFER-AC-3) — a fresh re-derivation, not
	// a call into the package's own computeLineTotals, so this test proves
	// the formula rather than the store's self-consistency.
	n, tx := roundThenSum(t, 1, 201, 50, 50)
	wantNet, wantTax := n+n, tx+tx // = 202, 102

	got, err := adapters.NewOfferStore(db).Get(context.Background(), offer.ID, wsID)
	if err != nil {
		t.Fatalf("get offer after lines: %v", err)
	}
	if got.NetMinor != wantNet || got.TaxMinor != wantTax || got.GrossMinor != wantNet+wantTax {
		t.Fatalf("reconciliation mismatch: got net=%d tax=%d gross=%d, want net=%d tax=%d gross=%d (a sum-then-round regression would instead yield net=201 tax=101)",
			got.NetMinor, got.TaxMinor, got.GrossMinor, wantNet, wantTax, wantNet+wantTax)
	}
}

// roundThenSum is the test's independent reference implementation of
// OFFER-PARAM-4, deliberately re-derived (not calling the package's own
// computeLineTotals) so this test actually proves the formula, not just
// that the store calls itself consistently.
func roundThenSum(t *testing.T, quantity float64, unitPriceMinor int64, discountPct, taxRate float64) (net, tax int64) {
	t.Helper()
	n := math.Round(quantity * float64(unitPriceMinor) * (1 - discountPct/100))
	tx := math.Round(n * taxRate / 100)
	return int64(n), int64(tx)
}

func TestOfferLineItemStore_Create_PositionConflict_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)

	base := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "A", Quantity: 1, UnitPriceMinor: i64Ptr(100), Source: "test", CapturedBy: "human:test",
	}
	if _, err := lineStore.Create(context.Background(), base, nil); err != nil {
		t.Fatalf("first create: %v", err)
	}
	dup := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "B", Quantity: 1, UnitPriceMinor: i64Ptr(100), Source: "test", CapturedBy: "human:test",
	}
	_, err := lineStore.Create(context.Background(), dup, nil)
	var posErr *adapters.ErrDuplicatePosition
	if !errors.As(err, &posErr) {
		t.Fatalf("expected ErrDuplicatePosition, got %v", err)
	}
}

func TestOfferLineItemStore_Mutations_RejectedWhenOfferNotDraft(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "A", Quantity: 1, UnitPriceMinor: i64Ptr(100), Source: "test", CapturedBy: "human:test",
	}
	created, err := lineStore.Create(context.Background(), li, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`UPDATE offer SET status='sent' WHERE id=$1::uuid`, offer.ID); err != nil {
		t.Fatalf("force status=sent: %v", err)
	}
	if _, err := lineStore.Update(context.Background(), created.ID, offer.ID, wsID, map[string]any{"description": "nope"}); !errors.Is(err, adapters.ErrOfferNotDraft) {
		t.Fatalf("update: expected ErrOfferNotDraft, got %v", err)
	}
	if err := lineStore.Delete(context.Background(), created.ID, offer.ID, wsID); !errors.Is(err, adapters.ErrOfferNotDraft) {
		t.Fatalf("delete: expected ErrOfferNotDraft, got %v", err)
	}
}

// TestOfferLineItemStore_Update_RecomputesOfferTotals is OFFER-WIRE-5's real
// assertion for the *successful* Update path (the only other Update call in
// this file, TestOfferLineItemStore_Mutations_RejectedWhenOfferNotDraft,
// only exercises the rejected offer-not-draft branch): a bounded partial
// update that changes quantity must recompute the parent offer's
// net_minor/tax_minor/gross_minor from the line's *new* quantity, not the
// pre-update one.
func TestOfferLineItemStore_Update_RecomputesOfferTotals(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "A", Quantity: 2, UnitPriceMinor: i64Ptr(1000),
		Source: "test", CapturedBy: "human:test",
	}
	created, err := lineStore.Create(context.Background(), li, f64Ptr(10))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := lineStore.Update(context.Background(), created.ID, offer.ID, wsID, map[string]any{"quantity": float64(5)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Quantity != 5 {
		t.Fatalf("expected updated line quantity=5, got %v", updated.Quantity)
	}

	// Independently compute expected totals with the same round-then-sum
	// formula the store must use, using the line's post-update quantity.
	wantNet, wantTax := roundThenSum(t, 5, 1000, 0, 10)

	got, err := adapters.NewOfferStore(db).Get(context.Background(), offer.ID, wsID)
	if err != nil {
		t.Fatalf("get offer after update: %v", err)
	}
	if got.NetMinor != wantNet || got.TaxMinor != wantTax || got.GrossMinor != wantNet+wantTax {
		t.Fatalf("expected offer totals recomputed from updated quantity: got net=%d tax=%d gross=%d, want net=%d tax=%d gross=%d",
			got.NetMinor, got.TaxMinor, got.GrossMinor, wantNet, wantTax, wantNet+wantTax)
	}
}

func TestOfferLineItemStore_Delete_HardDeletes_And_RecomputesTotals(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "A", Quantity: 2, UnitPriceMinor: i64Ptr(500), Source: "test", CapturedBy: "human:test",
	}
	created, err := lineStore.Create(context.Background(), li, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := lineStore.Delete(context.Background(), created.ID, offer.ID, wsID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM offer_line_item WHERE id=$1::uuid`, created.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatal("expected the row to be hard-deleted (0 rows), not soft-archived")
	}
	after, err := adapters.NewOfferStore(db).Get(context.Background(), offer.ID, wsID)
	if err != nil {
		t.Fatalf("get offer: %v", err)
	}
	if after.NetMinor != 0 || after.TaxMinor != 0 || after.GrossMinor != 0 {
		t.Fatalf("expected zero totals after deleting the only line, got net=%d tax=%d gross=%d", after.NetMinor, after.TaxMinor, after.GrossMinor)
	}
}

func TestOfferLineItemStore_Create_MissingProvenance_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID, dealID := seedOfferWorkspace(t, db)
	lineStore := adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db))
	offer := newDraftOffer(t, db, wsID, dealID)

	li := domain.OfferLineItem{
		WorkspaceID: wsID, OfferID: offer.ID, Position: 1,
		Description: "A", Quantity: 1, UnitPriceMinor: i64Ptr(100),
	}
	_, err := lineStore.Create(context.Background(), li, nil)
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("expected ErrNullProvenance, got %v", err)
	}
}
