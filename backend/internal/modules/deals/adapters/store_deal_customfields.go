package adapters

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/platform/customfields"
)

func (s *DealStore) ActiveCustomFieldNames(ctx context.Context, workspaceID string) ([]string, error) {
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, entityTypeDeal)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(active))
	for _, c := range active {
		names = append(names, c.ColumnName)
	}
	return names, nil
}
