package adapters

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
)

// DealFieldWriter applies accepted extraction fields to a deal using the
// deals module's existing partial-update path.
type DealFieldWriter struct {
	store *deals.DealStore
}

// NewDealFieldWriter wraps the deal store in the narrow seam transport needs.
func NewDealFieldWriter(store *deals.DealStore) *DealFieldWriter {
	return &DealFieldWriter{store: store}
}

// UpdateFields persists the accepted field map with last-write-wins semantics.
func (w *DealFieldWriter) UpdateFields(ctx context.Context, workspaceID, dealID string, updates map[string]any) error {
	_, err := w.store.Update(ctx, dealID, workspaceID, updates, 0)
	return err
}
