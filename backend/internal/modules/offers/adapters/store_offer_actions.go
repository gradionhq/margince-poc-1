package adapters

// store_offer_actions.go holds the offer state-transition verbs
// (render/send/accept, OFFER-WIRE-6/7/9) — split out of store_offer.go to
// stay under the 500-LOC-per-file cap (architecture/18 §3.2) once these
// landed alongside OP-T05's CRUD methods. Regenerate (OFFER-WIRE-8) lives in
// store_offer_regenerate.go, split out for the same reason once Accept
// landed here.

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

// Field/payload key literals repeated across Accept's DealStore.Update map,
// event_outbox payloads, and audit before/after maps (goconst).
const (
	fieldAmountMinor   = "amount_minor"
	fieldCurrency      = "currency"
	fieldCorrelationID = "correlation_id"
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
			fieldAmountMinor: offer.GrossMinor,
			fieldCurrency:    offer.Currency,
		}, deal.Version); err != nil {
			return fmt.Errorf("offer accept deal sync: %w", err)
		}

		if err := insertAcceptEvents(ctx, tx, workspaceID, id, offer, correlationID); err != nil {
			return err
		}
		return writeAcceptAudit(ctx, tx, workspaceID, id, status, now, offer, deal, correlationID)
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// insertAcceptEvents writes the offer.accepted and deal.updated event_outbox
// rows for a completed Accept, sharing correlationID so downstream consumers
// can join them. Extracted from Accept purely to keep its own length under
// the funlen gate; no behavior change.
func insertAcceptEvents(ctx context.Context, tx *sql.Tx, workspaceID, id string, offer domain.Offer, correlationID string) error {
	offerPayload, _ := json.Marshal(map[string]any{
		payloadKeyOfferID:  id,
		payloadKeyDealID:   offer.DealID,
		"revision":         offer.Revision,
		"gross_minor":      offer.GrossMinor,
		fieldCorrelationID: correlationID,
	})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "offer.accepted", id, offerPayload); err != nil {
		return fmt.Errorf("offer accepted event: %w", err)
	}

	dealPayload, _ := json.Marshal(map[string]any{
		payloadKeyDealID:   offer.DealID,
		fieldAmountMinor:   offer.GrossMinor,
		fieldCurrency:      offer.Currency,
		fieldCorrelationID: correlationID,
	})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "deal.updated", offer.DealID, dealPayload); err != nil {
		return fmt.Errorf("deal updated event: %w", err)
	}
	return nil
}

// writeAcceptAudit writes the paired offer and deal audit_log rows for a
// completed Accept (offer status transition, deal amount/currency sync),
// both carrying correlationID in their after-state. Extracted from Accept
// purely to keep its own length under the funlen gate; no behavior change.
func writeAcceptAudit(ctx context.Context, tx *sql.Tx, workspaceID, id, status string, now time.Time, offer domain.Offer, deal deals.Deal, correlationID string) error {
	oe := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOffer, &id,
		map[string]any{"status": status},
		map[string]any{"status": domain.OfferStatusAccepted, "accepted_at": now, fieldCorrelationID: correlationID})
	oe.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, oe); err != nil {
		return fmt.Errorf("offer accept audit: %w", err)
	}

	de := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeDeal, &offer.DealID,
		map[string]any{fieldAmountMinor: deal.AmountMinor, fieldCurrency: deal.Currency},
		map[string]any{fieldAmountMinor: offer.GrossMinor, fieldCurrency: offer.Currency, fieldCorrelationID: correlationID})
	de.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, de); err != nil {
		return fmt.Errorf("deal accept audit: %w", err)
	}
	return nil
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
