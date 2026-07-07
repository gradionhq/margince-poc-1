// Package ports defines the repository seams for the activities module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
)

// Store is the activity repository seam. Adapters implement this interface;
// the application service depends on it.
type Store interface {
	// Create inserts a new activity row and returns the persisted record.
	Create(ctx context.Context, a domain.Activity) (domain.Activity, error)
	// Get returns one activity by id, workspace-scoped; ErrNotFound if absent.
	Get(ctx context.Context, id, workspaceID string) (domain.Activity, error)
	// List returns a keyset page of activities, optionally filtered to a linked
	// entity, and the next cursor.
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error)
	// Update applies partial updates to an activity. When ifMatch==0 the version
	// check is skipped (last-write-wins). Returns the updated Activity.
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Activity, error)
	// Archive soft-deletes an activity (sets archived_at).
	Archive(ctx context.Context, id, workspaceID string) (domain.Activity, error)
}
