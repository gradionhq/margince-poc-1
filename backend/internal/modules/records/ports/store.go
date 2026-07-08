// Package ports defines the repository + scanner seams for the records module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
)

// AttachmentStore is the attachment repository seam.
type AttachmentStore interface {
	Create(ctx context.Context, a domain.Attachment) (domain.Attachment, error)
	// Get returns the raw row with no visibility/download-audit side effects
	// applied — the transport layer applies Constraint 6/5 on top of this.
	// ErrNotFound if absent or archived; use GetAny to also see archived rows.
	Get(ctx context.Context, id, workspaceID string) (domain.Attachment, error)
	// GetAny returns the raw row regardless of archived_at status — used by the
	// single-item GET read path so an archived attachment stays retrievable
	// (disclosed-locked 200) instead of 404ing.
	GetAny(ctx context.Context, id, workspaceID string) (domain.Attachment, error)
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int, includeArchived bool) ([]domain.Attachment, string, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Attachment, error)
	MarkScanResult(ctx context.Context, id, workspaceID string, scanner Scanner) (domain.Attachment, error)
}
