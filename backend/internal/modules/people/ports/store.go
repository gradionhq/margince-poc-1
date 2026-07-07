// Package ports defines the storage interfaces for the people module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/people/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

// PersonStorer is the persistence seam for person rows. It covers the full
// CRUD surface plus the merge, archive/restore, and strength-breakdown reads
// provided by adapters.PersonStore.
type PersonStorer interface {
	Create(ctx context.Context, p domain.Person, emails []domain.PersonEmailInput) (domain.Person, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Person, error)
	GetAny(ctx context.Context, id, workspaceID string) (domain.Person, error)
	List(ctx context.Context, workspaceID, cursor string, limit int, sort string) ([]domain.Person, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Person, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Person, error)
	Restore(ctx context.Context, id, workspaceID string) (domain.Person, error)
	Merge(ctx context.Context, loserID, targetID, workspaceID string) (domain.Person, error)
	StrengthBreakdown(ctx context.Context, id, workspaceID string) (strength.Result, error)
}
