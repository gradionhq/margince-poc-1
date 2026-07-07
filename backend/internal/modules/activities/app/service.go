// Package app contains the activities application service.
package app

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/modules/activities/ports"
)

// Service is the activities application service. It delegates all operations
// to the injected Store port.
type Service struct{ store ports.Store }

// NewService returns a new Service backed by the given Store.
func NewService(s ports.Store) *Service { return &Service{store: s} }

// Get returns one activity by id, workspace-scoped.
func (s *Service) Get(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	return s.store.Get(ctx, id, workspaceID)
}

// List returns a keyset page of activities, optionally filtered to a linked
// entity, and the next cursor.
func (s *Service) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error) {
	return s.store.List(ctx, workspaceID, entityType, entityID, cursor, limit)
}

// Update applies partial updates to an activity. When ifMatch==0 the version
// check is skipped (last-write-wins).
func (s *Service) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Activity, error) {
	return s.store.Update(ctx, id, workspaceID, updates, ifMatch)
}

// Archive soft-deletes an activity (sets archived_at).
func (s *Service) Archive(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	return s.store.Archive(ctx, id, workspaceID)
}
