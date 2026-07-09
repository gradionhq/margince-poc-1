package customfields

import (
	"strconv"
	"strings"

	"github.com/lib/pq"
)

// SelectSuffix returns the comma-prefixed, quoted custom-column list to
// append to a fixed SELECT column list (empty when there are no active
// columns), so a read path fetches its cf_* values in the same round trip as
// its fixed columns. Shared by person/organization/deal Get/List queries: the
// column-list-building mechanics are identical across resources even though
// the surrounding SELECT/Scan shape (and the domain struct it feeds) stays
// per-resource.
func SelectSuffix(active []Column) string {
	if len(active) == 0 {
		return ""
	}
	parts := make([]string, len(active))
	for i, c := range active {
		parts[i] = pq.QuoteIdentifier(c.ColumnName)
	}
	return ", " + strings.Join(parts, ", ")
}

// InsertColumns returns the quoted column names, $N placeholders, and bind
// args for each active custom column present (with a type-matching value) in
// rawExtra. nextParam is the first free bind-parameter index. A key with no
// active-column match, or whose value shape does not match the column type,
// is silently dropped (additionalProperties carries no per-key shape
// contract). Shared by person/organization Create — the mechanical
// column/placeholder/arg accumulation is identical across resources.
func InsertColumns(active []Column, rawExtra map[string]any, nextParam int) (cols, placeholders []string, args []any) {
	for _, c := range active {
		v, present := rawExtra[c.ColumnName]
		if !present {
			continue
		}
		sv, ok := SQLValue(c, v)
		if !ok {
			continue
		}
		cols = append(cols, pq.QuoteIdentifier(c.ColumnName))
		placeholders = append(placeholders, "$"+strconv.Itoa(nextParam+len(args)))
		args = append(args, sv)
	}
	return cols, placeholders, args
}

// UpdateSetClauses returns the "<col> = $N" SET-clause fragments and bind
// args for each active custom column present (with a type-matching value) in
// updates. nextParam is the first free bind-parameter index. Same
// drop-on-mismatch rule as InsertColumns. Shared by person/organization
// Update for the same reason as InsertColumns.
func UpdateSetClauses(active []Column, updates map[string]any, nextParam int) (clauses []string, args []any) {
	for _, c := range active {
		v, present := updates[c.ColumnName]
		if !present {
			continue
		}
		sv, ok := SQLValue(c, v)
		if !ok {
			continue
		}
		clauses = append(clauses, pq.QuoteIdentifier(c.ColumnName)+" = $"+strconv.Itoa(nextParam+len(args)))
		args = append(args, sv)
	}
	return clauses, args
}
