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
