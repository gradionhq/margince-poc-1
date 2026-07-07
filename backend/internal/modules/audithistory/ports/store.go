// Package ports defines the persistence seams for the audithistory module.
package ports

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
)

// HistoryReader is the read seam for audit history entries.
type HistoryReader interface {
	ReadHistory(ctx context.Context, entityType, entityID, workspaceID string) ([]domain.AuditHistoryEntry, error)
}
