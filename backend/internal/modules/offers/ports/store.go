// Package ports defines the repository seams for the offers module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
)

// ProductStore is the product repository seam.
type ProductStore interface {
	Create(ctx context.Context, p domain.Product) (domain.Product, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Product, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.Product, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Product, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Product, error)
}

// OfferTemplateStore is the offer_template repository seam.
type OfferTemplateStore interface {
	Create(ctx context.Context, t domain.OfferTemplate) (domain.OfferTemplate, error)
	Get(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, includeArchived bool) ([]domain.OfferTemplate, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.OfferTemplate, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.OfferTemplate, error)
}

// OfferStore is the offer repository seam.
type OfferStore interface {
	Create(ctx context.Context, o domain.Offer) (domain.Offer, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Offer, error)
	List(ctx context.Context, workspaceID, dealID, cursor string, limit int, includeArchived bool) ([]domain.Offer, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Offer, error)
	Regenerate(ctx context.Context, id, workspaceID string, signals []domain.OfferLineSignal) (domain.Offer, error)
}

// OfferLineItemStore is the offer_line_item repository seam.
type OfferLineItemStore interface {
	Create(ctx context.Context, li domain.OfferLineItem, explicitTaxRate *float64) (domain.OfferLineItem, error)
	List(ctx context.Context, offerID, workspaceID string) ([]domain.OfferLineItem, error)
	Update(ctx context.Context, id, offerID, workspaceID string, updates map[string]any) (domain.OfferLineItem, error)
	Delete(ctx context.Context, id, offerID, workspaceID string) error
}
