package adapters

// store_offer_actions.go holds the offer state-transition verbs
// (render/send/regenerate, OFFER-WIRE-6/7/8) — split out of store_offer.go
// to stay under the 500-LOC-per-file cap (architecture/18 §3.2) once these
// landed alongside OP-T05's CRUD methods.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/modules/organizations"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// ErrOfferNotSent reports a mutation attempted against an offer that is not sent.
var ErrOfferNotSent = errors.New("offer is not sent")

// ErrOfferNotAcceptable reports an accept attempted against an offer that is
// not currently sent.
var ErrOfferNotAcceptable = errors.New("offer is not acceptable")

// requireSent returns ErrOfferNotSent unless status is sent.
func requireSent(status string) error {
	if status != domain.OfferStatusSent {
		return ErrOfferNotSent
	}
	return nil
}

// loadWorkspaceNameAndBaseCurrency returns workspace.name and workspace.base_currency.
func loadWorkspaceNameAndBaseCurrency(ctx context.Context, tx *sql.Tx, workspaceID string) (name, baseCurrency string, err error) {
	err = tx.QueryRowContext(ctx, `
		SELECT name, base_currency FROM workspace WHERE id=$1::uuid`,
		workspaceID).Scan(&name, &baseCurrency)
	return name, baseCurrency, err
}

// buildBuyerSnapshot returns a stable buyer snapshot for send/render flows.
func (s *OfferStore) buildBuyerSnapshot(ctx context.Context, buyerOrgID *string, workspaceID string) (map[string]any, error) {
	if buyerOrgID == nil {
		return map[string]any{}, nil
	}
	org, err := organizations.NewOrgStore(s.db).Get(ctx, *buyerOrgID, workspaceID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"organization_id": org.ID,
		"display_name":    org.DisplayName,
		"address":         org.Address,
	}, nil
}

// RenderIngredients bundles the resolved inputs the PDF renderer needs.
// OfferStore stays free of blobstore concerns; the transport layer writes
// the PDF bytes and then persists the blob reference separately.
type RenderIngredients struct {
	Offer      domain.Offer
	LineItems  []domain.OfferLineItem
	BuyerBlock map[string]any
	IssuerName string
	Locale     string
}

// Send freezes the offer's FX metadata and buyer/issuer snapshots and marks it sent.
func (s *OfferStore) Send(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, _, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if err := requireDraft(status); err != nil {
			return err
		}

		var currency, dealID string
		var buyerOrgID sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT currency, deal_id, buyer_org_id FROM offer
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&currency, &dealID, &buyerOrgID); err != nil {
			return err
		}

		workspaceName, baseCurrency, err := loadWorkspaceNameAndBaseCurrency(ctx, tx, workspaceID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		rate := 1.0
		if currency != baseCurrency {
			rate, err = deals.AsOfFXRate(ctx, tx, workspaceID, currency, baseCurrency, now)
			if err != nil {
				return err
			}
		}

		var buyerOrgPtr *string
		if buyerOrgID.Valid {
			buyerOrgPtr = &buyerOrgID.String
		}
		buyerSnap, err := s.buildBuyerSnapshot(ctx, buyerOrgPtr, workspaceID)
		if err != nil {
			return err
		}

		buyerJSON, err := json.Marshal(buyerSnap)
		if err != nil {
			return err
		}
		issuerJSON, err := json.Marshal(map[string]any{"workspace_id": workspaceID, "name": workspaceName})
		if err != nil {
			return err
		}
		rateStr := strconv.FormatFloat(rate, 'f', 10, 64)

		if _, err := tx.ExecContext(ctx, `
			UPDATE offer SET status=$1, fx_rate_to_base=$2, fx_rate_date=$3,
			    buyer_snapshot=$4, issuer_snapshot=$5
			WHERE id=$6::uuid AND workspace_id=$7::uuid`,
			domain.OfferStatusSent, rateStr, now, buyerJSON, issuerJSON, id, workspaceID); err != nil {
			return fmt.Errorf("offer send: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{payloadKeyOfferID: id, payloadKeyDealID: dealID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "offer.sent", id, payload); err != nil {
			return fmt.Errorf("offer send event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOffer, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer send audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Accept flips a sent offer to accepted, syncs the parent deal through the
// deals module's public write path, and writes paired outbox/audit rows.
func (s *OfferStore) Accept(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	if s.dealStore == nil {
		return domain.Offer{}, fmt.Errorf("offers: Accept requires a deal store (call OfferStore.WithDealStore first)")
	}

	correlationID := ids.New()
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, _, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if status != domain.OfferStatusSent {
			return ErrOfferNotAcceptable
		}

		offer, err := scanOffer(tx.QueryRowContext(ctx, offerGetQuery, id, workspaceID))
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			UPDATE offer SET status=$1, accepted_at=$2
			WHERE id=$3::uuid AND workspace_id=$4::uuid`,
			domain.OfferStatusAccepted, now, id, workspaceID); err != nil {
			return fmt.Errorf("offer accept: %w", err)
		}

		deal, err := s.dealStore.Get(ctx, offer.DealID, workspaceID)
		if err != nil {
			return fmt.Errorf("offer accept deal lookup: %w", err)
		}
		if _, err := s.dealStore.Update(ctx, offer.DealID, workspaceID, map[string]any{
			"amount_minor": offer.GrossMinor,
			"currency":     offer.Currency,
		}, deal.Version); err != nil {
			return fmt.Errorf("offer accept deal sync: %w", err)
		}

		offerPayload, _ := json.Marshal(map[string]any{
			payloadKeyOfferID: id,
			payloadKeyDealID:  offer.DealID,
			"revision":        offer.Revision,
			"gross_minor":     offer.GrossMinor,
			"correlation_id":  correlationID,
		})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "offer.accepted", id, offerPayload); err != nil {
			return fmt.Errorf("offer accepted event: %w", err)
		}

		dealPayload, _ := json.Marshal(map[string]any{
			payloadKeyDealID: offer.DealID,
			"amount_minor":   offer.GrossMinor,
			"currency":       offer.Currency,
			"correlation_id": correlationID,
		})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "deal.updated", offer.DealID, dealPayload); err != nil {
			return fmt.Errorf("deal updated event: %w", err)
		}

		oe := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOffer, &id,
			map[string]any{"status": status},
			map[string]any{"status": domain.OfferStatusAccepted, "accepted_at": now, "correlation_id": correlationID})
		oe.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, oe); err != nil {
			return fmt.Errorf("offer accept audit: %w", err)
		}

		de := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeDeal, &offer.DealID,
			map[string]any{"amount_minor": deal.AmountMinor, "currency": deal.Currency},
			map[string]any{"amount_minor": offer.GrossMinor, "currency": offer.Currency, "correlation_id": correlationID})
		de.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, de); err != nil {
			return fmt.Errorf("deal accept audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, id, workspaceID)
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

// PrepareRender gathers the resolved, persisted inputs required to build an
// offer PDF without touching blob storage.
func (s *OfferStore) PrepareRender(ctx context.Context, id, workspaceID string) (RenderIngredients, error) {
	var out RenderIngredients
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, _, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}

		offer, err := scanOffer(tx.QueryRowContext(ctx, offerGetQuery, id, workspaceID))
		if err != nil {
			return err
		}
		out.Offer = offer

		rows, err := tx.QueryContext(ctx, offerLineItemListQuery, id, workspaceID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			li, scanErr := scanOfferLineItem(rows)
			if scanErr != nil {
				return scanErr
			}
			out.LineItems = append(out.LineItems, li)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		switch {
		case offer.Status == domain.OfferStatusSent:
			out.BuyerBlock = offer.BuyerSnapshot
		case offer.BuyerOrgID != nil:
			buyerBlock, err := s.buildBuyerSnapshot(ctx, offer.BuyerOrgID, workspaceID)
			if err != nil {
				return err
			}
			out.BuyerBlock = buyerBlock
		default:
			out.BuyerBlock = nil
		}

		issuerName, _, err := loadWorkspaceNameAndBaseCurrency(ctx, tx, workspaceID)
		if err != nil {
			return err
		}
		out.IssuerName = issuerName

		locale := localeDE
		if offer.TemplateID != nil {
			tmpl, err := NewOfferTemplateStore(s.db).Get(ctx, *offer.TemplateID, workspaceID)
			if err != nil {
				return err
			}
			locale = tmpl.Locale
		}
		out.Locale = locale
		return nil
	})
	if err != nil {
		return RenderIngredients{}, err
	}
	return out, nil
}

// SetPdfAssetRef stores the blob reference produced by render and audits it
// as a standard offer update.
func (s *OfferStore) SetPdfAssetRef(ctx context.Context, id, workspaceID, ref string) (domain.Offer, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		if _, _, err := lockOfferForMutation(ctx, tx, id, workspaceID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE offer SET pdf_asset_ref=$1
			WHERE id=$2::uuid AND workspace_id=$3::uuid`,
			ref, id, workspaceID); err != nil {
			return fmt.Errorf("offer render: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOffer, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer render audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, id, workspaceID)
}
