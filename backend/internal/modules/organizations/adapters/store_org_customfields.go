package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/platform/customfields"
)

// ActiveCustomFieldNames returns the workspace's active custom-column wire keys
// (e.g. ["cf_org_note"]) for the organization object — the thin passthrough the
// transport handler merges into its sort/filter vocabulary without importing
// customfields directly.
func (s *OrgStore) ActiveCustomFieldNames(ctx context.Context, workspaceID string) ([]string, error) {
	cols, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "organization")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.ColumnName
	}
	return names, nil
}

// cfSelectSuffix returns the comma-prefixed, quoted custom-column list to append
// to a fixed SELECT column list (empty when there are no active columns), so a
// read path fetches its cf_* values in the same round trip as its fixed columns.
func cfSelectSuffix(cols []customfields.Column) string {
	if len(cols) == 0 {
		return ""
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = pq.QuoteIdentifier(c.ColumnName)
	}
	return ", " + strings.Join(parts, ", ")
}

// cfInsertColumns returns the quoted column names, $N placeholders, and bind
// args for each active custom column present (with a type-matching value) in
// rawExtra. nextParam is the first free bind-parameter index. A key with no
// active-column match, or whose value shape does not match the column type, is
// silently dropped (additionalProperties carries no per-key shape contract).
func cfInsertColumns(active []customfields.Column, rawExtra map[string]any, nextParam int) (cols, placeholders []string, args []any) {
	for _, c := range active {
		v, present := rawExtra[c.ColumnName]
		if !present {
			continue
		}
		sv, ok := customfields.SQLValue(c, v)
		if !ok {
			continue
		}
		cols = append(cols, pq.QuoteIdentifier(c.ColumnName))
		placeholders = append(placeholders, fmt.Sprintf("$%d", nextParam+len(args)))
		args = append(args, sv)
	}
	return cols, placeholders, args
}

// cfUpdateSetClauses returns the "<col> = $N" SET-clause fragments and bind args
// for each active custom column present (with a type-matching value) in updates.
// nextParam is the first free bind-parameter index. Same drop-on-mismatch rule
// as cfInsertColumns.
func cfUpdateSetClauses(active []customfields.Column, updates map[string]any, nextParam int) (clauses []string, args []any) {
	for _, c := range active {
		v, present := updates[c.ColumnName]
		if !present {
			continue
		}
		sv, ok := customfields.SQLValue(c, v)
		if !ok {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s = $%d", pq.QuoteIdentifier(c.ColumnName), nextParam+len(args)))
		args = append(args, sv)
	}
	return clauses, args
}
