package adapters

import (
	"context"
	"strconv"
	"strings"

	"github.com/lib/pq"

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

func personCustomSelect(active []customfields.Column) string {
	if len(active) == 0 {
		return ""
	}
	cols := make([]string, len(active))
	for i, c := range active {
		cols[i] = pq.QuoteIdentifier(c.ColumnName)
	}
	return ", " + strings.Join(cols, ", ")
}

func personCustomInsert(active []customfields.Column, values map[string]any, start int) (string, string, []any) {
	cols := make([]string, 0, len(active))
	holders := make([]string, 0, len(active))
	args := make([]any, 0, len(active))
	for _, c := range active {
		v, ok := values[c.ColumnName]
		if !ok {
			continue
		}
		val, ok := customfields.SQLValue(c, v)
		if !ok {
			continue
		}
		cols = append(cols, pq.QuoteIdentifier(c.ColumnName))
		holders = append(holders, "$"+strconv.Itoa(start+len(args)))
		args = append(args, val)
	}
	if len(cols) == 0 {
		return "", "", nil
	}
	return ", " + strings.Join(cols, ", "), ", " + strings.Join(holders, ", "), args
}

func personCustomUpdate(active []customfields.Column, updates map[string]any, start int) (string, []any) {
	sets := make([]string, 0, len(active))
	args := make([]any, 0, len(active))
	for _, c := range active {
		v, ok := updates[c.ColumnName]
		if !ok {
			continue
		}
		val, ok := customfields.SQLValue(c, v)
		if !ok {
			continue
		}
		args = append(args, val)
		sets = append(sets, pq.QuoteIdentifier(c.ColumnName)+" = $"+strconv.Itoa(start+len(args)-1))
	}
	if len(sets) == 0 {
		return "", nil
	}
	return strings.Join(sets, ", "), args
}
