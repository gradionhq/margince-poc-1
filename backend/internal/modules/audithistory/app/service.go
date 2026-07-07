// Package app contains the audithistory module's application-service layer.
package app

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	"github.com/gradionhq/margince/backend/internal/modules/audithistory/ports"
)

// Service is the audithistory application service.
// It delegates to the HistoryReader port, providing a thin use-case boundary
// for future cross-cutting concerns (caching, pagination, authorization enrichment).
type Service struct {
	reader ports.HistoryReader
}

// New returns a new Service backed by the given HistoryReader.
func New(reader ports.HistoryReader) *Service {
	return &Service{reader: reader}
}

// ReadHistory returns all audit history entries for the given entity.
func (s *Service) ReadHistory(ctx context.Context, entityType, entityID, workspaceID string) ([]domain.AuditHistoryEntry, error) {
	return s.reader.ReadHistory(ctx, entityType, entityID, workspaceID)
}
