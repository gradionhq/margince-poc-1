// Package ports defines the storage interfaces for the relationships module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
)

// Store is the persistence seam for relationship rows.
type Store interface {
	Create(ctx context.Context, rel domain.Relationship) (domain.Relationship, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Relationship, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, filter domain.RelationshipListFilter) ([]domain.Relationship, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Relationship, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Relationship, error)
}
