// Package ports defines the storage interfaces for the partners module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/partners/domain"
)

// Store is the persistence seam for partner rows.
type Store interface {
	Upsert(ctx context.Context, p domain.Partner) (domain.Partner, error)
	Get(ctx context.Context, organizationID, workspaceID string) (domain.Partner, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, filter domain.PartnerListFilter) ([]domain.Partner, string, error)
}
