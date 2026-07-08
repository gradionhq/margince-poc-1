// Package ports defines the storage interfaces for the organizations module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
)

// OrgStorer is the persistence seam for organization rows.
type OrgStorer interface {
	Create(ctx context.Context, o domain.Organization) (domain.Organization, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Organization, error)
	GetAny(ctx context.Context, id, workspaceID string) (domain.Organization, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Organization, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Organization, error)
	Restore(ctx context.Context, id, workspaceID string) (domain.Organization, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, sortVal string, filter domain.OrgListFilter) ([]domain.Organization, string, error)
	Merge(ctx context.Context, loserID, targetID, workspaceID string) (domain.Organization, error)
}
