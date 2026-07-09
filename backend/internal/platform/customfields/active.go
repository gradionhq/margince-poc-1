package customfields

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/platform/database"
)

// Column is one active custom-field column for a (workspace, object) pair.
type Column struct {
	ColumnName string
	Slug       string
	Type       string
}

// ActiveColumns reads the active custom-field catalog rows for one object.
func ActiveColumns(ctx context.Context, db *sql.DB, workspaceID, object string) ([]Column, error) {
	var cols []Column
	err := database.WithWorkspaceTx(ctx, db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT column_name, slug, type FROM custom_field WHERE workspace_id=$1::uuid AND object=$2 AND status='active' ORDER BY column_name`, workspaceID, object)
		if err != nil {
			return fmt.Errorf("customfields: select active columns: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var c Column
			if err := rows.Scan(&c.ColumnName, &c.Slug, &c.Type); err != nil {
				return fmt.Errorf("customfields: scan active column: %w", err)
			}
			cols = append(cols, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("customfields: iterate active columns: %w", err)
		}
		return nil
	})
	return cols, err
}

// ScanDests returns fresh scan destinations for active columns.
func ScanDests(active []Column) []any {
	dests := make([]any, len(active))
	for i := range active {
		var v any
		dests[i] = &v
	}
	return dests
}

// ExtractValues converts scanned custom-field values into wire values.
func ExtractValues(active []Column, dests []any) map[string]any {
	out := make(map[string]any, len(active))
	for i, c := range active {
		if i >= len(dests) || dests[i] == nil {
			continue
		}
		p, ok := dests[i].(*any)
		if !ok || p == nil || *p == nil {
			continue
		}
		if v, ok := extractOne(c.Type, *p); ok {
			out[c.ColumnName] = v
		}
	}
	return out
}

// extractOne converts one raw driver-scanned value v into its wire
// representation for column type t, per the type-specific conversion rules
// ExtractValues documents (currency/boolean passthrough, text/picklist
// []byte->string, date bare "2006-01-02" string, number arbitrary-precision
// json.Number). ok=false when v's Go type doesn't match t's expected shape.
func extractOne(t string, v any) (any, bool) {
	switch t {
	case TypeCurrency:
		n, ok := v.(int64)
		return n, ok
	case TypeBoolean:
		b, ok := v.(bool)
		return b, ok
	case TypeText, TypePicklist:
		switch s := v.(type) {
		case []byte:
			return string(s), true
		case string:
			return s, true
		}
	case TypeDate:
		switch d := v.(type) {
		case time.Time:
			return d.Format("2006-01-02"), true
		case string:
			return d, true
		}
	case TypeNumber:
		switch n := v.(type) {
		case []byte:
			return json.Number(string(n)), true
		case string:
			return json.Number(n), true
		}
	}
	return nil, false
}

// SQLValue converts one JSON-decoded value into a database bind value.
func SQLValue(c Column, v any) (any, bool) {
	switch c.Type {
	case TypeCurrency:
		f, ok := v.(float64)
		if !ok {
			return nil, false
		}
		return int64(f), true
	case TypeNumber:
		f, ok := v.(float64)
		if !ok {
			return nil, false
		}
		return f, true
	case TypeDate:
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		return s, true
	case TypeBoolean:
		b, ok := v.(bool)
		if !ok {
			return nil, false
		}
		return b, true
	case TypeText, TypePicklist:
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		return s, true
	default:
		return nil, false
	}
}
