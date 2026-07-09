package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// aiDisclosureText is the Art. 50 disclosure surfaced on every regenerate
// response that actually applied AI-authored lines (GATE-AI-9) — never set
// when a regenerate call found no grounded signals to apply (evidence-or-
// omit, no-fabrication: OfferStore.Regenerate in store_offer_actions.go only
// stamps this onto the result when aiApplied is true).
const aiDisclosureText = "This offer draft was generated with AI assistance from the deal's captured context. Review every proposed line and price before sending."

// buildAndInsertRegeneratedLine builds one AI-drafted domain.OfferLineItem
// from signal, resolves its price (grounded from the signal itself, or
// falling back to the linked product's rate-card price when the signal
// carries no price), inserts the row, and echoes Source/CapturedBy back onto
// the scanned result (not DB-backed columns — see
// OfferLineItemStore.Create's own doc comment for the same pattern).
// Extracted from Regenerate purely to keep its WithWorkspaceTx closure's
// cognitive complexity under the gate; no behavior change.
func (s *OfferStore) buildAndInsertRegeneratedLine(ctx context.Context, tx *sql.Tx, products *ProductStore, newID, workspaceID string, position int, signal domain.OfferLineSignal) (domain.OfferLineItem, error) {
	li := domain.OfferLineItem{
		ID:          ids.New(),
		WorkspaceID: workspaceID,
		OfferID:     newID,
		Position:    position,
		ProductID:   signal.ProductID,
		Description: signal.Description,
		Unit:        defaultUnit,
		Quantity:    signal.Quantity,
		Source:      "api",
		CapturedBy:  "agent:offer-drafting",
		Evidence:    &domain.Evidence{Snippet: signal.Snippet, SourceID: signal.SourceID},
	}
	switch {
	case signal.UnitPriceMinor != nil:
		li.UnitPriceMinor = *signal.UnitPriceMinor
		li.PriceGrounded = true
	case signal.ProductID != nil:
		product, err := products.Get(ctx, *signal.ProductID, workspaceID)
		if err != nil {
			if !errors.Is(err, errs.ErrNotFound) {
				return domain.OfferLineItem{}, err
			}
			break
		}
		li.UnitPriceMinor = product.UnitPriceMinor
		li.PriceGrounded = true
		if product.DefaultTaxRate != nil {
			li.TaxRate = *product.DefaultTaxRate
		}
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO offer_line_item (id, workspace_id, offer_id, position, product_id,
		    description, unit, quantity, unit_price_minor, price_grounded, discount_pct,
		    tax_rate, evidence)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING
			id, workspace_id, offer_id, position, product_id, description, unit,
			quantity, unit_price_minor, price_grounded, discount_pct, tax_rate, evidence,
			created_at, updated_at, archived_at`,
		li.ID, li.WorkspaceID, li.OfferID, li.Position, li.ProductID, li.Description, li.Unit,
		li.Quantity, li.UnitPriceMinor, li.PriceGrounded, li.DiscountPct, li.TaxRate, evidenceJSON(li.Evidence))
	created, err := scanOfferLineItem(row)
	if err != nil {
		return domain.OfferLineItem{}, fmt.Errorf("offer regenerate line insert: %w", err)
	}
	// Echo back source/captured_by from input (validated but not DB-backed) —
	// mirrors OfferLineItemStore.Create's own established pattern.
	created.Source, created.CapturedBy = li.Source, li.CapturedBy
	return created, nil
}

func listLineItemsTx(ctx context.Context, tx *sql.Tx, offerID, workspaceID string) ([]domain.OfferLineItem, error) {
	rows, err := tx.QueryContext(ctx, offerLineItemListQuery, offerID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []domain.OfferLineItem{}
	for rows.Next() {
		li, scanErr := scanOfferLineItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, li)
	}
	return out, rows.Err()
}

func computeOfferDiff(prior, next []domain.OfferLineItem) *domain.OfferDiff {
	diff := &domain.OfferDiff{
		Added:   []domain.OfferLineItem{},
		Removed: []domain.OfferLineItem{},
		Changed: []domain.OfferLineItemChange{},
	}
	priorByDescription := make(map[string]domain.OfferLineItem, len(prior))
	for _, line := range prior {
		priorByDescription[line.Description] = line
	}
	seen := make(map[string]struct{}, len(next))
	for _, line := range next {
		seen[line.Description] = struct{}{}
		before, ok := priorByDescription[line.Description]
		if !ok {
			diff.Added = append(diff.Added, line)
			continue
		}
		if lineChanged(before, line) {
			diff.Changed = append(diff.Changed, domain.OfferLineItemChange{Before: before, After: line})
		}
	}
	for _, line := range prior {
		if _, ok := seen[line.Description]; !ok {
			diff.Removed = append(diff.Removed, line)
		}
	}
	return diff
}

func lineChanged(a, b domain.OfferLineItem) bool {
	return a.Quantity != b.Quantity ||
		a.UnitPriceMinor != b.UnitPriceMinor ||
		a.DiscountPct != b.DiscountPct ||
		a.TaxRate != b.TaxRate ||
		a.PriceGrounded != b.PriceGrounded
}
