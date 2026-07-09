package adapters

// store_offer_regenerate.go holds OfferStore.Regenerate (OFFER-WIRE-8) and
// its helpers — split out of store_offer_actions.go to stay under the
// 500-LOC-per-file cap (architecture/18 §3.2) once Accept (OFFER-WIRE-9)
// landed there.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
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

// cloneOfferForRegenerate inserts newOffer (id/revision/status already set
// by the caller, the rest copied verbatim from the source row) as a clone of
// the offer row id/workspaceID, and clones that offer's live line items onto
// the new offer id. Called from Regenerate inside its own tx.
func cloneOfferForRegenerate(ctx context.Context, tx *sql.Tx, newOffer domain.Offer, id, workspaceID string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO offer (
			id, workspace_id, deal_id, offer_number, revision, status, currency,
			buyer_org_id, buyer_snapshot, issuer_snapshot, valid_until, intro_text,
			terms_text, net_minor, tax_minor, gross_minor, fx_rate_to_base,
			fx_rate_date, template_id, pdf_asset_ref, accepted_at, version,
			source, captured_by
		)
		SELECT
			$1::uuid, workspace_id, deal_id, offer_number, $2, $3, currency,
			buyer_org_id, buyer_snapshot, issuer_snapshot, valid_until, intro_text,
			terms_text, net_minor, tax_minor, gross_minor, fx_rate_to_base,
			fx_rate_date, template_id, NULL, NULL, 1,
			source, captured_by
		FROM offer
		WHERE id=$4::uuid AND workspace_id=$5::uuid`,
		newOffer.ID, newOffer.Revision, newOffer.Status, id, workspaceID); err != nil {
		return fmt.Errorf("offer regenerate insert: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO offer_line_item (
			id, workspace_id, offer_id, position, product_id, description, unit,
			quantity, unit_price_minor, discount_pct, tax_rate
		)
		SELECT
			uuidv7(), workspace_id, $1::uuid, position, product_id, description, unit,
			quantity, unit_price_minor, discount_pct, tax_rate
		FROM offer_line_item
		WHERE offer_id=$2::uuid AND workspace_id=$3::uuid AND archived_at IS NULL`,
		newOffer.ID, id, workspaceID); err != nil {
		return fmt.Errorf("offer regenerate lines: %w", err)
	}
	return nil
}

// Regenerate always starts from the verbatim clone of a sent offer's live
// line items + row (OFFER-AC-10d) into a new draft revision, marking the
// prior revision superseded. When signals carries at least one grounded
// AI-proposed line (domain.FilterGroundedSignals — evidence-or-omit), the
// cloned lines are discarded and replaced with the AI-authored lines
// instead, totals are recomputed, and a diff against the prior revision's
// lines is computed; AIGenerated/AIDisclosure/DiffFromPrevious are only set
// on the result in that case — a plain regenerate (nil/no-grounded-signals,
// e.g. today's NoOpRetriever-backed real callers) never claims AI
// involvement and never silently drops the prior line items (no-fabrication
// / evidence-or-omit, shared by OP-T06 and OP-T07).
func (s *OfferStore) Regenerate(ctx context.Context, id, workspaceID string, signals []domain.OfferLineSignal) (domain.Offer, error) {
	grounded := domain.FilterGroundedSignals(signals)
	products := NewProductStore(s.db)

	var out domain.Offer
	var diff *domain.OfferDiff
	aiApplied := false

	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, version, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if err := requireSent(status); err != nil {
			return err
		}

		orig, err := scanOffer(tx.QueryRowContext(ctx, offerGetQuery, id, workspaceID))
		if err != nil {
			return err
		}

		priorLines, err := listLineItemsTx(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}

		out = nextRegenerateRevision(orig)
		if err := cloneOfferForRegenerate(ctx, tx, out, id, workspaceID); err != nil {
			return err
		}

		if len(grounded) > 0 {
			aiApplied = true
			diff, err = s.applyAIRegenerate(ctx, tx, products, out.ID, workspaceID, grounded, priorLines)
			if err != nil {
				return err
			}
		}

		return supersedeAndAuditRegenerate(ctx, tx, id, workspaceID, version, orig, out)
	})
	if err != nil {
		return domain.Offer{}, err
	}

	result, err := s.Get(ctx, out.ID, workspaceID)
	if err != nil {
		return domain.Offer{}, err
	}
	if aiApplied {
		result.AIGenerated = true
		disclosure := aiDisclosureText
		result.AIDisclosure = &disclosure
		result.DiffFromPrevious = diff
	}
	return result, nil
}

// nextRegenerateRevision derives the new draft revision's domain.Offer value
// from orig (same deal/offer_number/currency/buyer/terms/provenance, fresh
// id, revision+1, version reset to 1, send/render/archive fields cleared).
// Extracted from Regenerate purely to keep its own length under the funlen
// gate; no behavior change.
func nextRegenerateRevision(orig domain.Offer) domain.Offer {
	out := orig
	out.ID = ids.New()
	out.Status = domain.OfferStatusDraft
	out.Revision = orig.Revision + 1
	out.Version = 1
	out.AcceptedAt = nil
	out.PdfAssetRef = nil
	out.CreatedAt = time.Time{}
	out.UpdatedAt = time.Time{}
	out.ArchivedAt = nil
	return out
}

// applyAIRegenerate replaces newID's cloned line items with the AI-authored
// lines built from grounded, recomputes newID's totals (only this path
// recomputes — the verbatim-clone path already carries over the prior row's
// exact net/tax/gross via cloneOfferForRegenerate's SELECT clone), and
// returns the diff against priorLines. Extracted from Regenerate purely to
// keep its own length under the funlen gate; no behavior change.
func (s *OfferStore) applyAIRegenerate(ctx context.Context, tx *sql.Tx, products *ProductStore, newID, workspaceID string, grounded []domain.OfferLineSignal, priorLines []domain.OfferLineItem) (*domain.OfferDiff, error) {
	newLines, err := s.replaceClonedLinesWithAI(ctx, tx, products, newID, workspaceID, grounded)
	if err != nil {
		return nil, err
	}
	if err := recomputeOfferTotals(ctx, tx, newID, workspaceID); err != nil {
		return nil, err
	}
	return computeOfferDiff(priorLines, newLines), nil
}

// supersedeAndAuditRegenerate flips id's status to superseded (optimistic
// version bump), writes its offer.superseded outbox event (entity_id=id —
// the entity that was superseded), and writes out.ID's create audit_log
// entry — all under the caller's workspace tx, regardless of which
// Regenerate path (verbatim clone or AI-applied) ran. Extracted from
// Regenerate purely to keep its own length under the funlen gate; no
// behavior change.
func supersedeAndAuditRegenerate(ctx context.Context, tx *sql.Tx, id, workspaceID string, version int64, orig, out domain.Offer) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE offer SET status=$2, version=$3
		WHERE id=$1::uuid AND workspace_id=$4::uuid`,
		id, domain.OfferStatusSuperseded, version+1, workspaceID); err != nil {
		return fmt.Errorf("offer regenerate supersede: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		payloadKeyOfferID: id,
		payloadKeyDealID:  orig.DealID,
		"new_offer_id":    out.ID,
		"from_revision":   orig.Revision,
		"to_revision":     out.Revision,
	})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "offer.superseded", id, payload); err != nil {
		return fmt.Errorf("offer regenerate event: %w", err)
	}

	e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOffer, &out.ID, nil, out)
	e.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
		return fmt.Errorf("offer regenerate audit: %w", err)
	}
	return nil
}

// replaceClonedLinesWithAI discards newID's just-cloned line items (hard
// delete — they were only ever visible inside this same transaction, never
// returned to a caller) and inserts one AI-drafted line per grounded signal
// in their place, in position order, returning the persisted lines for the
// diff computation. Only called when grounded is non-empty.
func (s *OfferStore) replaceClonedLinesWithAI(ctx context.Context, tx *sql.Tx, products *ProductStore, newID, workspaceID string, grounded []domain.OfferLineSignal) ([]domain.OfferLineItem, error) {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM offer_line_item WHERE offer_id=$1::uuid AND workspace_id=$2::uuid`,
		newID, workspaceID); err != nil {
		return nil, fmt.Errorf("offer regenerate clear cloned lines: %w", err)
	}
	newLines := make([]domain.OfferLineItem, 0, len(grounded))
	for i, signal := range grounded {
		created, err := s.buildAndInsertRegeneratedLine(ctx, tx, products, newID, workspaceID, i+1, signal)
		if err != nil {
			return nil, err
		}
		newLines = append(newLines, created)
	}
	return newLines, nil
}
