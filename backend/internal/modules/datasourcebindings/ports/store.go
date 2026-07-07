// Package ports defines the storage interfaces the datasourcebindings module depends on.
// These are narrow seams — only the methods the DatasourceProvider actually calls are
// included. The concrete implementations live in the entity-specific modules (people,
// deals, etc.) and satisfy these interfaces at wire time.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/domain"
)

// PersonStorer is the persistence seam for person records as needed by the
// datasource binding.
type PersonStorer interface {
	Create(ctx context.Context, p domain.Person, emails []domain.PersonEmailInput) (domain.Person, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Person, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Person, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, sort string) ([]domain.Person, string, error)
}

// OrgStorer is the persistence seam for organization records as needed by the
// datasource binding.
type OrgStorer interface {
	Create(ctx context.Context, o domain.Organization) (domain.Organization, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Organization, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Organization, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, sort string, filter domain.OrgListFilter) ([]domain.Organization, string, error)
}

// DealStorer is the persistence seam for deal records as needed by the
// datasource binding.
type DealStorer interface {
	Create(ctx context.Context, d domain.Deal, idempotencyKey string) (domain.Deal, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Deal, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Deal, error)
	List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Deal, string, error)
}

// ActivityStorer is the persistence seam for activity records as needed by the
// datasource binding.
type ActivityStorer interface {
	Create(ctx context.Context, a domain.Activity) (domain.Activity, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Activity, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Activity, error)
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error)
}

// LeadStorer is the persistence seam for lead records as needed by the
// datasource binding.
type LeadStorer interface {
	Create(ctx context.Context, l domain.Lead) (domain.Lead, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Lead, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Lead, error)
	List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Lead, string, error)
}
