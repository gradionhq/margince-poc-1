package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// aiDisclosureText is the Art. 50 disclosure surfaced on every regenerate
// response (GATE-AI-9) — this ticket originates the pattern for this
// surface only.
const aiDisclosureText = "This offer draft was generated with AI assistance from the deal's captured context. Review every proposed line and price before sending."

// Regenerate assembles, filters, prices, diffs, persists, and supersedes in
// one workspace-scoped transaction. Signals arrive already decoded from the
// transport seam; this method stays on plain domain data and never imports
// retrieval.
func (s *OfferStore) Regenerate(ctx context.Context, id, workspaceID string, signals []domain.OfferLineSignal) (domain.Offer, error) {
	grounded := domain.FilterGroundedSignals(signals)
	newID := ids.New()
	products := NewProductStore(s.db)

	var diff *domain.OfferDiff
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, _, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if err := requireDraft(status); err != nil {
			return err
		}

		row := tx.QueryRowContext(ctx, offerGetQuery, id, workspaceID)
		prior, err := scanOffer(row)
		if err != nil {
			return err
		}

		priorLines, err := listLineItemsTx(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}

		if err := insertRegeneratedOfferHeader(ctx, tx, newID, workspaceID, prior); err != nil {
			return err
		}

		newLines := make([]domain.OfferLineItem, 0, len(grounded))
		for i, signal := range grounded {
			created, err := s.buildAndInsertRegeneratedLine(ctx, tx, products, newID, workspaceID, i+1, signal)
			if err != nil {
				return err
			}
			newLines = append(newLines, created)
		}

		if err := recomputeOfferTotals(ctx, tx, newID, workspaceID); err != nil {
			return err
		}

		if err := supersedeAndAuditRegenerate(ctx, tx, id, newID, workspaceID, prior); err != nil {
			return err
		}

		diff = computeOfferDiff(priorLines, newLines)
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}

	result, err := s.Get(ctx, newID, workspaceID)
	if err != nil {
		return domain.Offer{}, err
	}
	result.AIGenerated = true
	disclosure := aiDisclosureText
	result.AIDisclosure = &disclosure
	result.DiffFromPrevious = diff
	return result, nil
}

// insertRegeneratedOfferHeader inserts newID's offer row as a new draft
// revision cloned from prior (same deal/offer_number/currency/buyer/terms,
// revision+1, AI provenance). Extracted from Regenerate purely to keep its
// WithWorkspaceTx closure's length/complexity under the gate; no behavior
// change.
func insertRegeneratedOfferHeader(ctx context.Context, tx *sql.Tx, newID, workspaceID string, prior domain.Offer) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO offer (id, workspace_id, deal_id, offer_number, revision, status, currency,
		    buyer_org_id, valid_until, intro_text, terms_text, template_id, source, captured_by, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1)`,
		newID, workspaceID, prior.DealID, prior.OfferNumber, prior.Revision+1, domain.OfferStatusDraft,
		prior.Currency, prior.BuyerOrgID, prior.ValidUntil, prior.IntroText, prior.TermsText,
		prior.TemplateID, "api", "agent:offer-drafting"); err != nil {
		return fmt.Errorf("offer regenerate insert: %w", err)
	}
	return nil
}

// supersedeAndAuditRegenerate flips id's status to superseded, writes its
// offer.superseded outbox event, and writes newID's create audit_log entry —
// all under the caller's workspace tx. Extracted from Regenerate purely to
// keep its WithWorkspaceTx closure's length/complexity under the gate; no
// behavior change.
func supersedeAndAuditRegenerate(ctx context.Context, tx *sql.Tx, id, newID, workspaceID string, prior domain.Offer) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE offer SET status=$1 WHERE id=$2::uuid AND workspace_id=$3::uuid`,
		domain.OfferStatusSuperseded, id, workspaceID); err != nil {
		return fmt.Errorf("offer regenerate supersede: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"offer_id":      id,
		"deal_id":       prior.DealID,
		"from_revision": prior.Revision,
		"to_revision":   prior.Revision + 1,
	})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "offer.superseded", id, payload); err != nil {
		return fmt.Errorf("offer regenerate event: %w", err)
	}

	entry := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOffer, &newID, nil, nil)
	entry.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, entry); err != nil {
		return fmt.Errorf("offer regenerate audit: %w", err)
	}
	return nil
}

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
