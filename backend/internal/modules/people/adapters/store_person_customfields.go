package adapters

import (
	"context"
	"strings"

	"github.com/gradionhq/margince/backend/internal/platform/customfields"
)

// ActiveCustomFieldNames returns the workspace's active cf_* column names on
// person, for merging into the sort-vocabulary allow-list.
func (s *PersonStore) ActiveCustomFieldNames(ctx context.Context, workspaceID string) ([]string, error) {
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "person")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(active))
	for i, c := range active {
		names[i] = c.ColumnName
	}
	return names, nil
}

// personCustomInsert adapts customfields.InsertColumns's slice results into
// the comma-prefixed joined-string shape Create's literal INSERT statement
// wants (organization's Create instead appends the slices directly, so it
// calls customfields.InsertColumns itself without this wrapper).
func personCustomInsert(active []customfields.Column, values map[string]any, start int) (string, string, []any) {
	cols, holders, args := customfields.InsertColumns(active, values, start)
	if len(cols) == 0 {
		return "", "", nil
	}
	return ", " + strings.Join(cols, ", "), ", " + strings.Join(holders, ", "), args
}

// personCustomUpdate adapts customfields.UpdateSetClauses's slice result into
// the comma-joined SET-clause fragment string Update's literal UPDATE
// statement wants (organization appends the slice directly).
func personCustomUpdate(active []customfields.Column, updates map[string]any, start int) (string, []any) {
	sets, args := customfields.UpdateSetClauses(active, updates, start)
	if len(sets) == 0 {
		return "", nil
	}
	return strings.Join(sets, ", "), args
}
