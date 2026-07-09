package customfields

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExtractValues(t *testing.T) {
	col := func(name, typ string) Column {
		return Column{ColumnName: name, Type: typ}
	}
	now := time.Date(2026, 7, 9, 12, 34, 56, 0, time.UTC)
	cases := []struct {
		name string
		cols []Column
		dsts []any
		want map[string]any
	}{
		{
			name: "currency",
			cols: []Column{col("cf_currency", TypeCurrency)},
			dsts: []any{ptrAny(any(int64(1234)))},
			want: map[string]any{"cf_currency": int64(1234)},
		},
		{
			name: "boolean",
			cols: []Column{col("cf_boolean", TypeBoolean)},
			dsts: []any{ptrAny(any(true))},
			want: map[string]any{"cf_boolean": true},
		},
		{
			name: "text",
			cols: []Column{col("cf_text", TypeText)},
			dsts: []any{ptrAny(any([]byte("hello")))},
			want: map[string]any{"cf_text": "hello"},
		},
		{
			name: "picklist",
			cols: []Column{col("cf_picklist", TypePicklist)},
			dsts: []any{ptrAny(any([]byte("direct")))},
			want: map[string]any{"cf_picklist": "direct"},
		},
		{
			name: "date",
			cols: []Column{col("cf_date", TypeDate)},
			dsts: []any{ptrAny(any(now))},
			want: map[string]any{"cf_date": "2026-07-09"},
		},
		{
			name: "number",
			cols: []Column{col("cf_number", TypeNumber)},
			dsts: []any{ptrAny(any([]byte("1234.500")))},
			want: map[string]any{"cf_number": json.Number("1234.500")},
		},
		{
			name: "omits_nil_dest",
			cols: []Column{col("cf_text", TypeText)},
			dsts: []any{nil},
			want: map[string]any{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractValues(tc.cols, tc.dsts)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d want %d got=%v", len(got), len(tc.want), got)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Fatalf("key %s: got %#v want %#v", k, got[k], want)
				}
			}
			for _, c := range tc.cols {
				if _, ok := got[c.ColumnName]; !ok && tc.want[c.ColumnName] != nil {
					t.Fatalf("expected key %s to be present", c.ColumnName)
				}
			}
		})
	}
}

func TestSQLValue(t *testing.T) {
	cases := []struct {
		name string
		col  Column
		in   any
		want any
		ok   bool
	}{
		{name: "currency", col: Column{Type: TypeCurrency}, in: float64(1234.9), want: int64(1234), ok: true},
		{name: "number", col: Column{Type: TypeNumber}, in: float64(12.5), want: float64(12.5), ok: true},
		{name: "date", col: Column{Type: TypeDate}, in: "2026-07-09", want: "2026-07-09", ok: true},
		{name: "boolean", col: Column{Type: TypeBoolean}, in: true, want: true, ok: true},
		{name: "text", col: Column{Type: TypeText}, in: "hello", want: "hello", ok: true},
		{name: "picklist", col: Column{Type: TypePicklist}, in: "direct", want: "direct", ok: true},
		{name: "currency_wrong_shape", col: Column{Type: TypeCurrency}, in: "1234", want: nil, ok: false},
		{name: "date_wrong_shape", col: Column{Type: TypeDate}, in: true, want: nil, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SQLValue(tc.col, tc.in)
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v", ok, tc.ok)
			}
			if !tc.ok {
				if got != nil {
					t.Fatalf("expected zero value, got %#v", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
}

func ptrAny(v any) *any { return &v }
