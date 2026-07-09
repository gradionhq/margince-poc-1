// Package ports defines the storage interfaces for the deals module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
)

// DealStorer is the persistence seam for deal rows.
type DealStorer interface {
	Create(ctx context.Context, d domain.Deal, idempotencyKey string, rawExtra map[string]any) (domain.Deal, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Deal, error)
	GetAny(ctx context.Context, id, workspaceID string) (domain.Deal, error)
	FindByIdempotencyKey(ctx context.Context, workspaceID, key string) (domain.Deal, bool, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Deal, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Deal, error)
	Restore(ctx context.Context, id, workspaceID string) (domain.Deal, error)
	StageSemantic(ctx context.Context, stageID, workspaceID string) (string, error)
	Advance(ctx context.Context, id, workspaceID string, in domain.AdvanceInput, ifMatch int64, changedBy string) (domain.Deal, error)
	List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Deal, string, error)
	ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, f domain.DealListFilter) ([]domain.Deal, string, error)
}
