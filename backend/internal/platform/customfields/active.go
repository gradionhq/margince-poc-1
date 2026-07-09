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

// ExtractValues converts scanned custom-field values into wire values. A NULL
// column (nil dest) is omitted from the map entirely.
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
		if wire, ok := wireValue(c.Type, *p); ok {
			out[c.ColumnName] = wire
		}
	}
	return out
}

// wireValue converts one scanned driver value into its wire representation for
// the given custom-field type, returning ok=false when the value's Go type does
// not match the column type (the key is then omitted from the response).
func wireValue(fieldType string, raw any) (any, bool) {
	switch fieldType {
	case TypeCurrency:
		v, ok := raw.(int64)
		return v, ok
	case TypeBoolean:
		v, ok := raw.(bool)
		return v, ok
	case TypeText, TypePicklist:
		switch v := raw.(type) {
		case []byte:
			return string(v), true
		case string:
			return v, true
		}
	case TypeDate:
		switch v := raw.(type) {
		case time.Time:
			return v.Format("2006-01-02"), true
		case string:
			return v, true
		}
	case TypeNumber:
		switch v := raw.(type) {
		case []byte:
			return json.Number(string(v)), true
		case string:
			return json.Number(v), true
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
