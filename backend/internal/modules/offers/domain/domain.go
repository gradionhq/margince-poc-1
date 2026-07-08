// Package domain holds the Product and OfferTemplate entities (OFFER-DDL-1,
// OFFER-DDL-4) — the offers module's two flat, independent catalog/config
// tables. No cross-entity composite type: product and offer_template share a
// module only because both are OFFER-DDL-owned contract-adjacent tables, not
// because either references the other.
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// Product is a rate-card / catalogue entry (OFFER-DDL-1). unit_price_minor is
// integer minor-units — never a float, on the wire or in Go (OFFER-AC-9a).
type Product struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspace_id"`
	Name           string     `json:"name"`
	SKU            *string    `json:"sku"`
	Description    *string    `json:"description"`
	Unit           *string    `json:"unit"`
	UnitPriceMinor int64      `json:"unit_price_minor"`
	Currency       string     `json:"currency"`
	DefaultTaxRate *float64   `json:"default_tax_rate"`
	Active         bool       `json:"active"`
	Version        int64      `json:"version"`
	Source         string     `json:"source"`
	CapturedBy     string     `json:"captured_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ArchivedAt     *time.Time `json:"archived_at"`
}

// NewProduct returns a Product with a fresh ID, version 1, and copied
// provenance, defaulting active=true and unit="unit" (DB defaults mirrored in
// Go so a freshly-constructed value already matches a round-tripped row).
func NewProduct(name string, p prov.Provenance) Product {
	unit := "unit"
	return Product{
		ID: ids.New(), Name: name, Unit: &unit, Active: true,
		Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// OfferTemplate is a branded, governed PDF layout for offers (OFFER-DDL-4).
// Source/CapturedBy are carried here to match the crm.yaml wire schema, but
// the offer_template table has no such columns (the DDL's own comment: "not
// captured data") — OfferTemplateStore validates their presence on create
// (422 if empty, mirroring every other module's provenance gate) but never
// persists them; Get/List/Update/Archive always return them as "".
type OfferTemplate struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	Name        string                 `json:"name"`
	Locale      string                 `json:"locale"`
	IsDefault   bool                   `json:"is_default"`
	Layout      map[string]interface{} `json:"layout"`
	Source      string                 `json:"source"`
	CapturedBy  string                 `json:"captured_by"`
	Version     int64                  `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ArchivedAt  *time.Time             `json:"archived_at"`
}

// NewOfferTemplate returns an OfferTemplate with a fresh ID, version 1, and
// the request's source/captured_by echoed (validated, never persisted — see
// the type doc comment), defaulting locale="de-DE" (DDL default).
func NewOfferTemplate(name string, p prov.Provenance) OfferTemplate {
	return OfferTemplate{
		ID: ids.New(), Name: name, Locale: "de-DE",
		Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// OfferStatusDraft is the only status this ticket's Update/line-item paths
// ever accept a mutation against (OFFER-WIRE-4 draft-only guard).
const OfferStatusDraft = "draft"

// OfferStatusSent marks an offer that has been frozen and sent.
const OfferStatusSent = "sent"

// OfferStatusSuperseded marks a sent offer that has been replaced by a new revision.
const OfferStatusSuperseded = "superseded"

// Offer is a versioned Angebot bound to one deal (OFFER-DDL-2). net_minor/
// tax_minor/gross_minor are DERIVED server-side from line items
// (OFFER-PARAM-4) — never accepted from the client (API-ERR-15).
// BuyerSnapshot/IssuerSnapshot/FxRateToBase/FxRateDate/PdfAssetRef/AcceptedAt
// are all owned by the out-of-scope send/render/accept verbs and stay nil/
// zero through this ticket's entire lifecycle.
type Offer struct {
	ID             string                 `json:"id"`
	WorkspaceID    string                 `json:"workspace_id"`
	DealID         string                 `json:"deal_id"`
	OfferNumber    string                 `json:"offer_number"`
	Revision       int64                  `json:"revision"`
	Status         string                 `json:"status"`
	Currency       string                 `json:"currency"`
	BuyerOrgID     *string                `json:"buyer_org_id"`
	BuyerSnapshot  map[string]interface{} `json:"buyer_snapshot"`
	IssuerSnapshot map[string]interface{} `json:"issuer_snapshot"`
	ValidUntil     *time.Time             `json:"valid_until"`
	IntroText      *string                `json:"intro_text"`
	TermsText      *string                `json:"terms_text"`
	NetMinor       int64                  `json:"net_minor"`
	TaxMinor       int64                  `json:"tax_minor"`
	GrossMinor     int64                  `json:"gross_minor"`
	FxRateToBase   *string                `json:"fx_rate_to_base"`
	FxRateDate     *time.Time             `json:"fx_rate_date"`
	TemplateID     *string                `json:"template_id"`
	PdfAssetRef    *string                `json:"pdf_asset_ref"`
	AcceptedAt     *time.Time             `json:"accepted_at"`
	Version        int64                  `json:"version"`
	Source         string                 `json:"source"`
	CapturedBy     string                 `json:"captured_by"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	ArchivedAt     *time.Time             `json:"archived_at"`
}

// NewOffer returns an Offer with a fresh ID, status=draft, revision=1,
// version=1, and copied provenance.
func NewOffer(dealID, offerNumber, currency string, p prov.Provenance) Offer {
	return Offer{
		ID: ids.New(), DealID: dealID, OfferNumber: offerNumber, Currency: currency,
		Status: OfferStatusDraft, Revision: 1, Version: 1,
		Source: p.Source, CapturedBy: p.CapturedBy,
	}
}

// OfferLineItem is a typed line on an offer (OFFER-DDL-3); price is a
// snapshot copied from product at pick time (OFFER-AC-9b), never re-read
// later. No version column (no optimistic concurrency on line items).
type OfferLineItem struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspace_id"`
	OfferID        string     `json:"offer_id"`
	Position       int        `json:"position"`
	ProductID      *string    `json:"product_id"`
	Description    string     `json:"description"`
	Unit           string     `json:"unit"`
	Quantity       float64    `json:"quantity"`
	UnitPriceMinor int64      `json:"unit_price_minor"`
	DiscountPct    float64    `json:"discount_pct"`
	TaxRate        float64    `json:"tax_rate"`
	Source         string     `json:"source"`
	CapturedBy     string     `json:"captured_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ArchivedAt     *time.Time `json:"archived_at"`
}

// NewOfferLineItem returns an OfferLineItem with a fresh ID and copied
// provenance, defaulting unit="unit" (DDL default).
func NewOfferLineItem(offerID string, position int, description string, quantity float64, unitPriceMinor int64, p prov.Provenance) OfferLineItem {
	return OfferLineItem{
		ID: ids.New(), OfferID: offerID, Position: position, Description: description,
		Unit: "unit", Quantity: quantity, UnitPriceMinor: unitPriceMinor,
		Source: p.Source, CapturedBy: p.CapturedBy,
	}
}
