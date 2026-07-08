// Package ports defines the storage interfaces for the leads module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/leads/domain"
)

// LeadStorer is the persistence seam for lead rows.
type LeadStorer interface {
	// Create inserts a new lead; returns ErrLeadEmailDuplicate on uq_lead_email violation.
	Create(ctx context.Context, l domain.Lead) (domain.Lead, error)
	// Get returns a live lead by id+workspace; ErrNotFound if absent or archived.
	Get(ctx context.Context, id, workspaceID string) (domain.Lead, error)
	// List returns a keyset-paged slice of live leads and the next-page cursor.
	List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Lead, string, error)
	// Update applies partial updates (status, owner_id) with optional optimistic
	// concurrency check (ifMatch==0 skips the version guard).
	Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Lead, error)
	// Archive disqualifies a lead (status=disqualified + archived_at) and emits
	// lead.disqualified; a no-op re-archive emits no event.
	Archive(ctx context.Context, id, workspaceID string) (domain.Lead, error)
	// Promote converts a lead into a person in a single atomic transaction and
	// returns the newly-created person's ID.
	Promote(ctx context.Context, leadID, workspaceID, actorID string) (string, error)
}
