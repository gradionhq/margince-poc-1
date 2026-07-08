// Package offers is the offers domain module: the product rate-card
// (OFFER-DDL-1) and offer_template branded-layout admin surface (OFFER-DDL-4).
// The module exposes ProductStore/OfferTemplateStore adapters and
// ProductHandler/OfferTemplateHandler for HTTP routing.
package offers

import (
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/modules/offers/transport"
)

// Product is a type alias for domain.Product, re-exported so callers can
// refer to offers domain types via this package.
type Product = domain.Product

// OfferTemplate is a type alias for domain.OfferTemplate.
type OfferTemplate = domain.OfferTemplate

// ProductStore is a type alias for adapters.ProductStore.
type ProductStore = adapters.ProductStore

// OfferTemplateStore is a type alias for adapters.OfferTemplateStore.
type OfferTemplateStore = adapters.OfferTemplateStore

// NewProductStore returns a ProductStore backed by db.
func NewProductStore(db *sql.DB) *ProductStore { return adapters.NewProductStore(db) }

// NewOfferTemplateStore returns an OfferTemplateStore backed by db.
func NewOfferTemplateStore(db *sql.DB) *OfferTemplateStore { return adapters.NewOfferTemplateStore(db) }

// Module is the offers module's dependency-injection handle.
type Module struct {
	ProductStore         *adapters.ProductStore
	OfferTemplateStore   *adapters.OfferTemplateStore
	ProductHandler       *transport.ProductHandler
	OfferTemplateHandler *transport.OfferTemplateHandler
}

// New constructs the offers Module wiring both stores and both HTTP handlers.
func New(db *sql.DB) *Module {
	productStore := adapters.NewProductStore(db)
	offerTemplateStore := adapters.NewOfferTemplateStore(db)
	return &Module{
		ProductStore:         productStore,
		OfferTemplateStore:   offerTemplateStore,
		ProductHandler:       transport.NewProductHandler(productStore),
		OfferTemplateHandler: transport.NewOfferTemplateHandler(offerTemplateStore),
	}
}
