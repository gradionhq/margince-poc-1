package records

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	audithistorydomain "github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
)

// strp is a test helper that returns a pointer to s.
func strp(s string) *string { return &s }

// mustUnmarshal JSON-decodes src into map[string]any the same way the DB driver does.
func mustUnmarshal(t *testing.T, src string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("mustUnmarshal: %v", err)
	}
	return m
}

// baseRow returns a minimal auditLogRow for use in tests.
func baseRow() auditLogRow {
	return auditLogRow{
		id:         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		entityType: "organization",
		entityID:   "11111111-2222-3333-4444-555555555555",
		actorType:  "human",
		actorID:    "user-1",
		occurredAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
}

// noMask is an empty EntityFieldMask (nothing masked).
var noMask audithistorydomain.EntityFieldMask

// ---- diffRowFields tests ----

func TestDiffRowFields_MultiFieldMutation(t *testing.T) {
	// 3 keys changed, 1 unchanged — alphabetical order, shared id/changed_at
	row := baseRow()
	row.before = map[string]any{"alpha": "1", "beta": "2", "gamma": "3", "delta": "4"}
	row.after = map[string]any{"alpha": "X", "beta": "Y", "gamma": "Z", "delta": "4"}

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 3 {
		t.Fatalf("want 3 entries (delta unchanged), got %d", len(entries))
	}
	// alphabetical: alpha, beta, gamma
	wantFields := []string{"alpha", "beta", "gamma"}
	for i, e := range entries {
		if e.Field != wantFields[i] {
			t.Errorf("entry[%d].Field = %q, want %q", i, e.Field, wantFields[i])
		}
		if e.ID != row.id {
			t.Errorf("entry[%d].ID = %q, want %q", i, e.ID, row.id)
		}
		if !e.ChangedAt.Equal(row.occurredAt) {
			t.Errorf("entry[%d].ChangedAt = %v, want %v", i, e.ChangedAt, row.occurredAt)
		}
		if e.OldValue == nil {
			t.Errorf("entry[%d].OldValue is nil, want non-nil", i)
		}
		if e.NewValue == nil {
			t.Errorf("entry[%d].NewValue is nil, want non-nil", i)
		}
	}
}

func TestDiffRowFields_Create(t *testing.T) {
	// before=nil (create) — every after key emits with old_value=nil
	row := baseRow()
	row.before = nil
	row.after = map[string]any{"name": "Acme", "type": "enterprise"}

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.OldValue != nil {
			t.Errorf("entry %q: OldValue want nil, got %q", e.Field, *e.OldValue)
		}
		if e.NewValue == nil {
			t.Errorf("entry %q: NewValue is nil, want non-nil", e.Field)
		}
	}
}

func TestDiffRowFields_FieldRemoved(t *testing.T) {
	// field present in before but absent in after → new_value=nil
	row := baseRow()
	row.before = map[string]any{"website": "https://old.example.com"}
	row.after = map[string]any{}

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Field != "website" {
		t.Errorf("Field = %q, want %q", e.Field, "website")
	}
	if e.OldValue == nil || *e.OldValue == "" {
		t.Errorf("OldValue want non-nil non-empty, got %v", e.OldValue)
	}
	if e.NewValue != nil {
		t.Errorf("NewValue want nil for removed field, got %q", *e.NewValue)
	}
}

func TestDiffRowFields_ErasureTombstone(t *testing.T) {
	// both before and after nil (erasure tombstone) → zero entries, no panic
	row := baseRow()
	row.before = nil
	row.after = nil

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 0 {
		t.Fatalf("want 0 entries for erasure tombstone, got %d", len(entries))
	}
}

func TestDiffRowFields_MaskedKeyHidden(t *testing.T) {
	// masked key never appears in output even when it changed
	mask := audithistorydomain.EntityFieldMask{"secret": {}}
	row := baseRow()
	row.before = map[string]any{"name": "Alice", "secret": "pass1"}
	row.after = map[string]any{"name": "Bob", "secret": "pass2"}

	entries := diffRowFields(row, mask, nil)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry (name only), got %d: %+v", len(entries), entries)
	}
	if entries[0].Field != "name" {
		t.Errorf("Field = %q, want %q", entries[0].Field, "name")
	}
	for _, e := range entries {
		if e.Field == "secret" {
			t.Errorf("masked field 'secret' must not appear in output")
		}
	}
}

func TestDiffRowFields_FieldFilter(t *testing.T) {
	// fieldFilter narrows to only that key's entry
	row := baseRow()
	row.before = map[string]any{"a": "1", "b": "2"}
	row.after = map[string]any{"a": "2", "b": "3"}

	filter := "a"
	entries := diffRowFields(row, noMask, &filter)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry for field 'a', got %d", len(entries))
	}
	if entries[0].Field != "a" {
		t.Errorf("Field = %q, want %q", entries[0].Field, "a")
	}
}

func TestDiffRowFields_AgentActorAttribution(t *testing.T) {
	// actor_type=agent → passportID and evidence copied onto entries
	pid := "pppppppp-pppp-pppp-pppp-pppppppppppp"
	ev := map[string]any{"source": "memory"}

	row := baseRow()
	row.actorType = "agent"
	row.passportID = &pid
	row.evidence = ev
	row.before = nil
	row.after = map[string]any{"status": "active"}

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.PassportID == nil || *e.PassportID != pid {
		t.Errorf("PassportID = %v, want %q", e.PassportID, pid)
	}
	if e.Evidence == nil || e.Evidence["source"] != "memory" {
		t.Errorf("Evidence = %v, want map with source=memory", e.Evidence)
	}
}

func TestDiffRowFields_NonAgentActorNilsAgentFields(t *testing.T) {
	// non-agent actor → passportID and evidence are nil on entries
	// even if the row carries non-nil values (spec's explicit restriction)
	pid := "pppppppp-pppp-pppp-pppp-pppppppppppp"
	ev := map[string]any{"source": "memory"}

	for _, actorType := range []string{"human", "system", "connector"} {
		row := baseRow()
		row.actorType = actorType
		row.passportID = &pid
		row.evidence = ev
		row.before = nil
		row.after = map[string]any{"status": "active"}

		entries := diffRowFields(row, noMask, nil)

		if len(entries) != 1 {
			t.Fatalf("[%s] want 1 entry, got %d", actorType, len(entries))
		}
		e := entries[0]
		if e.PassportID != nil {
			t.Errorf("[%s] PassportID should be nil for non-agent, got %v", actorType, e.PassportID)
		}
		if e.Evidence != nil {
			t.Errorf("[%s] Evidence should be nil for non-agent, got %v", actorType, e.Evidence)
		}
	}
}

func TestDiffRowFields_Stringification(t *testing.T) {
	// non-nil value stringifies via %v; nil/absent → nil *string
	row := baseRow()
	row.before = nil
	row.after = map[string]any{
		"count":  float64(42),
		"label":  "hello",
		"nested": map[string]any{"x": float64(1)},
		"items":  []any{"a", "b"},
	}

	entries := diffRowFields(row, noMask, nil)

	// all 4 fields emitted (create)
	if len(entries) != 4 {
		t.Fatalf("want 4 entries, got %d", len(entries))
	}

	// verify each new_value is non-nil (not the literal "nil"/"<nil>") and old_value is nil
	for _, e := range entries {
		if e.OldValue != nil {
			t.Errorf("field %q: OldValue want nil, got %q", e.Field, *e.OldValue)
		}
		if e.NewValue == nil {
			t.Fatalf("field %q: NewValue is nil, want stringified", e.Field)
		}
		if *e.NewValue == "nil" || *e.NewValue == "<nil>" {
			t.Errorf("field %q: NewValue = %q, must not be literal nil string", e.Field, *e.NewValue)
		}
	}

	// verify float64 stringification
	byField := make(map[string]FieldHistoryEntry)
	for _, e := range entries {
		byField[e.Field] = e
	}
	if got := *byField["count"].NewValue; got != fmt.Sprintf("%v", float64(42)) {
		t.Errorf("count NewValue = %q, want %q", got, fmt.Sprintf("%v", float64(42)))
	}
}

func TestDiffRowFields_ReflectDeepEqualForMaps(t *testing.T) {
	// structurally-equal map values (that would panic with ==) must NOT emit an entry
	// This proves reflect.DeepEqual (not ==) is used for comparison.
	nestedBefore := map[string]any{"key": float64(1)}
	nestedAfter := map[string]any{"key": float64(1)} // same structure, different pointer

	row := baseRow()
	row.before = map[string]any{"data": nestedBefore}
	row.after = map[string]any{"data": nestedAfter}

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 0 {
		t.Fatalf("equal nested maps must emit 0 entries (reflect.DeepEqual), got %d", len(entries))
	}
}

func TestDiffRowFields_JSONDecodedValues(t *testing.T) {
	// JSON-decoded values (as the DB driver produces) stringify without panicking
	before := mustUnmarshal(t, `{"score": 3.14, "tags": ["a", "b"], "meta": {"k": "v"}}`)
	after := mustUnmarshal(t, `{"score": 9.99, "tags": ["a", "b", "c"], "meta": {"k": "v"}}`)

	row := baseRow()
	row.before = before
	row.after = after

	// score and tags changed; meta unchanged
	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 2 {
		t.Fatalf("want 2 changed entries (score, tags), got %d", len(entries))
	}
	for _, e := range entries {
		if e.NewValue == nil {
			t.Errorf("field %q: NewValue is nil", e.Field)
		}
	}
}

func TestDiffRowFields_NilAbsentStringifiesAsNilPointer(t *testing.T) {
	// nil/absent values → nil *string, not the strings "nil" or "<nil>"
	row := baseRow()
	// field 'x' present in before with value nil (JSON null), absent in after (field removed)
	row.before = map[string]any{"x": nil}
	row.after = map[string]any{}

	entries := diffRowFields(row, noMask, nil)

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.OldValue != nil {
		t.Errorf("OldValue: nil JSON value must stringify to nil *string, got %q", *e.OldValue)
	}
	if e.NewValue != nil {
		t.Errorf("NewValue: absent field must be nil *string, got %q", *e.NewValue)
	}
}

// ---- encodeCursor / decodeCursor tests ----

func TestCursorRoundTrip(t *testing.T) {
	ts := time.Date(2026, 3, 14, 15, 9, 26, 535897932, time.UTC)
	id := "cafecafe-cafe-cafe-cafe-cafecafecafe"

	encoded := encodeCursor(ts, id)
	got, gotID, ok := decodeCursor(encoded)
	if !ok {
		t.Fatalf("decodeCursor returned ok=false for valid cursor %q", encoded)
	}
	if !got.Equal(ts) {
		t.Errorf("occurredAt: got %v, want %v", got, ts)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
	}
}

func TestCursorBlank_FirstPage(t *testing.T) {
	// blank cursor → "no cursor", ok=true, zero time (first page)
	gotTime, gotID, ok := decodeCursor("")
	if !ok {
		t.Fatal("decodeCursor(\"\") must return ok=true (blank is not an error)")
	}
	if !gotTime.IsZero() {
		t.Errorf("gotTime want zero (no cursor), got %v", gotTime)
	}
	if gotID != "" {
		t.Errorf("gotID want empty, got %q", gotID)
	}
}

func TestCursorInvalidBase64(t *testing.T) {
	_, _, ok := decodeCursor("garbage-not-base64!!!")
	if ok {
		t.Fatal("decodeCursor with invalid base64 must return ok=false")
	}
}

func TestCursorValidBase64WrongShape(t *testing.T) {
	// syntactically valid base64 but missing the | separator
	bad := base64.RawURLEncoding.EncodeToString([]byte("no-pipe-here"))
	_, _, ok := decodeCursor(bad)
	if ok {
		t.Fatal("decodeCursor with valid base64 but wrong shape must return ok=false")
	}
}

func TestCursorValidBase64InvalidTimestamp(t *testing.T) {
	// valid base64, has |, but timestamp is not RFC3339Nano
	bad := base64.RawURLEncoding.EncodeToString([]byte("not-a-timestamp|some-id"))
	_, _, ok := decodeCursor(bad)
	if ok {
		t.Fatal("decodeCursor with invalid timestamp must return ok=false")
	}
}
