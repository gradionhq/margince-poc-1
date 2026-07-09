package customfields

import "testing"

func TestSelectSuffix(t *testing.T) {
	if got := SelectSuffix(nil); got != "" {
		t.Fatalf("empty active: got %q want %q", got, "")
	}
	cols := []Column{{ColumnName: "cf_note"}, {ColumnName: "cf_score"}}
	want := `, "cf_note", "cf_score"`
	if got := SelectSuffix(cols); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestInsertColumns(t *testing.T) {
	active := []Column{
		{ColumnName: "cf_note", Type: TypeText},
		{ColumnName: "cf_score", Type: TypeNumber},
		{ColumnName: "cf_missing", Type: TypeText},
	}
	rawExtra := map[string]any{
		"cf_note":       "hello",
		"cf_score":      float64(12.5),
		"cf_wrong_type": "ignored, no matching active column",
	}
	cols, placeholders, args := InsertColumns(active, rawExtra, 5)
	if len(cols) != 2 || cols[0] != `"cf_note"` || cols[1] != `"cf_score"` {
		t.Fatalf("cols = %#v", cols)
	}
	if len(placeholders) != 2 || placeholders[0] != "$5" || placeholders[1] != "$6" {
		t.Fatalf("placeholders = %#v", placeholders)
	}
	if len(args) != 2 || args[0] != "hello" || args[1] != float64(12.5) {
		t.Fatalf("args = %#v", args)
	}

	// cf_missing has no matching key in rawExtra, so it is silently dropped.
	cols2, _, args2 := InsertColumns(active, map[string]any{"cf_note": 42}, 1)
	if len(cols2) != 0 || len(args2) != 0 {
		t.Fatalf("shape-mismatched value should be dropped: cols=%#v args=%#v", cols2, args2)
	}
}

func TestUpdateSetClauses(t *testing.T) {
	active := []Column{
		{ColumnName: "cf_note", Type: TypeText},
		{ColumnName: "cf_score", Type: TypeNumber},
	}
	updates := map[string]any{
		"cf_note":  "updated",
		"cf_score": float64(1),
	}
	clauses, args := UpdateSetClauses(active, updates, 3)
	if len(clauses) != 2 || clauses[0] != `"cf_note" = $3` || clauses[1] != `"cf_score" = $4` {
		t.Fatalf("clauses = %#v", clauses)
	}
	if len(args) != 2 || args[0] != "updated" || args[1] != float64(1) {
		t.Fatalf("args = %#v", args)
	}

	if clauses, args := UpdateSetClauses(active, map[string]any{}, 1); len(clauses) != 0 || len(args) != 0 {
		t.Fatalf("no updates should yield no clauses/args: clauses=%#v args=%#v", clauses, args)
	}
}
