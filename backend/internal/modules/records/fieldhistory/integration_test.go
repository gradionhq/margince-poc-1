//go:build integration

package fieldhistory_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/lib/pq"

	audithistorydomain "github.com/gradionhq/margince/backend/internal/modules/audithistory/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/fieldhistory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// seedFieldHistoryRow inserts one audit_log row directly and returns its id.
// occurredAt == nil → DB default (now()); non-nil → explicit value (for ordering tests).
func seedFieldHistoryRow(
	t *testing.T,
	db *sql.DB,
	wsID, entityType, entityID, actorType, actorID string,
	passportID *string,
	evidence map[string]any,
	before, after map[string]any,
	occurredAt *time.Time,
) string {
	t.Helper()
	ctx := context.Background()

	marshalJSON := func(v map[string]any) []byte {
		if v == nil {
			return nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("seedFieldHistoryRow marshal: %v", err)
		}
		return b
	}

	beforeJSON := marshalJSON(before)
	afterJSON := marshalJSON(after)
	evJSON := marshalJSON(evidence)

	var rowID string
	var err error
	if occurredAt != nil {
		err = db.QueryRowContext(ctx, `
			INSERT INTO audit_log
			  (workspace_id, actor_type, actor_id, passport_id, action, entity_type, entity_id,
			   before, after, evidence, occurred_at)
			VALUES ($1::uuid, $2, $3, $4::uuid, 'update', $5, $6::uuid, $7, $8, $9, $10)
			RETURNING id`,
			wsID, actorType, actorID, passportID, entityType, entityID,
			beforeJSON, afterJSON, evJSON, occurredAt).Scan(&rowID)
	} else {
		err = db.QueryRowContext(ctx, `
			INSERT INTO audit_log
			  (workspace_id, actor_type, actor_id, passport_id, action, entity_type, entity_id,
			   before, after, evidence)
			VALUES ($1::uuid, $2, $3, $4::uuid, 'update', $5, $6::uuid, $7, $8, $9)
			RETURNING id`,
			wsID, actorType, actorID, passportID, entityType, entityID,
			beforeJSON, afterJSON, evJSON).Scan(&rowID)
	}
	if err != nil {
		t.Fatalf("seedFieldHistoryRow: %v", err)
	}
	return rowID
}

// strPtr returns a *string pointer to s.
func strPtr(s string) *string { return &s }

// TestStore_DiffProjection covers RD-AC-11/RD-WIRE-5:
// one audit_log row with a 3-field before/after diff (2 changed, 1 unchanged) produces
// exactly 2 entries sharing that row's id + changed_at; the unchanged field produces zero.
func TestStore_DiffProjection(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	entityID := ids.New()

	rowID := seedFieldHistoryRow(
		t, db, ws, "organization", entityID, "human", "user-1", nil, nil,
		map[string]any{"name": "Acme", "status": "active", "size": "large"},
		map[string]any{"name": "Acme Corp", "status": "active", "size": "medium"},
		nil,
	)

	store := fieldhistory.NewStore(db)
	entries, cursor, err := store.List(context.Background(), ws, "organization", entityID, nil, nil, "", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// 2 changed fields (name, size); status unchanged
	if len(entries) != 2 {
		t.Fatalf("want 2 entries (name, size changed; status unchanged), got %d: %+v", len(entries), entries)
	}
	if cursor != "" {
		t.Errorf("cursor want empty (exhausted), got %q", cursor)
	}

	// All entries share the row's id and changed_at
	for _, e := range entries {
		if e.ID != rowID {
			t.Errorf("entry.ID = %q, want %q (source audit_log row id)", e.ID, rowID)
		}
		if e.EntityType != "organization" {
			t.Errorf("entry.EntityType = %q, want organization", e.EntityType)
		}
		if e.EntityID != entityID {
			t.Errorf("entry.EntityID = %q, want %q", e.EntityID, entityID)
		}
	}

	// Fields are alphabetically ordered: name, size
	if entries[0].Field != "name" {
		t.Errorf("entries[0].Field = %q, want name", entries[0].Field)
	}
	if entries[1].Field != "size" {
		t.Errorf("entries[1].Field = %q, want size", entries[1].Field)
	}

	// name: Acme → Acme Corp
	if entries[0].OldValue == nil || *entries[0].OldValue != "Acme" {
		t.Errorf("name OldValue = %v, want Acme", entries[0].OldValue)
	}
	if entries[0].NewValue == nil || *entries[0].NewValue != "Acme Corp" {
		t.Errorf("name NewValue = %v, want Acme Corp", entries[0].NewValue)
	}
}

// TestStore_Attribution covers RD-AC-5:
// agent entries carry passport_id+evidence; human entries do not;
// actor_type filter narrows to matching rows; field filter narrows to one field.
func TestStore_Attribution(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	entityID := ids.New()
	passID := ids.New()
	ev := map[string]any{"source": "memory", "confidence": 0.9}

	// Agent row: two fields changed
	agentRowID := seedFieldHistoryRow(
		t, db, ws, "person", entityID, "agent", "agent-1", &passID, ev,
		map[string]any{"score": float64(1), "label": "low"},
		map[string]any{"score": float64(5), "label": "high"},
		nil,
	)
	_ = agentRowID

	// Human row: one field changed
	seedFieldHistoryRow(
		t, db, ws, "person", entityID, "human", "user-2", nil, nil,
		map[string]any{"label": "high"},
		map[string]any{"label": "vip"},
		nil,
	)

	store := fieldhistory.NewStore(db)
	ctx := context.Background()

	t.Run("agent_attribution", func(t *testing.T) {
		// All entries — agent entries carry passport_id + evidence
		entries, _, err := store.List(ctx, ws, "person", entityID, nil, nil, "", 50)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		// 2 from agent row (score, label) + 1 from human row (label) = 3 total
		if len(entries) != 3 {
			t.Fatalf("want 3 entries, got %d: %+v", len(entries), entries)
		}

		// Entries are newest-first; human row was inserted last so it's first
		// Verify the agent entries carry passport_id + evidence
		for _, e := range entries {
			if e.ActorType != "agent" {
				if e.PassportID != nil {
					t.Errorf("human entry: PassportID must be nil, got %v", e.PassportID)
				}
				if e.Evidence != nil {
					t.Errorf("human entry: Evidence must be nil, got %v", e.Evidence)
				}
				continue
			}
			if e.PassportID == nil || *e.PassportID != passID {
				t.Errorf("agent entry: PassportID = %v, want %q", e.PassportID, passID)
			}
			if e.Evidence == nil {
				t.Errorf("agent entry: Evidence is nil, want non-nil")
			}
		}
	})

	t.Run("actor_type_filter", func(t *testing.T) {
		entries, _, err := store.List(ctx, ws, "person", entityID, nil, strPtr("agent"), "", 50)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		// Only agent row's entries
		if len(entries) != 2 {
			t.Fatalf("actor_type=agent: want 2 entries, got %d", len(entries))
		}
		for _, e := range entries {
			if e.ActorType != "agent" {
				t.Errorf("filtered entry has actor_type %q, want agent", e.ActorType)
			}
			if e.PassportID == nil {
				t.Errorf("agent entry must carry passport_id")
			}
		}
	})

	t.Run("field_filter", func(t *testing.T) {
		entries, _, err := store.List(ctx, ws, "person", entityID, strPtr("label"), nil, "", 50)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		// Only entries for "label" field
		for _, e := range entries {
			if e.Field != "label" {
				t.Errorf("field-filtered entry has Field = %q, want label", e.Field)
			}
		}
		// Both rows changed "label", so 2 entries
		if len(entries) != 2 {
			t.Fatalf("field=label: want 2 entries, got %d", len(entries))
		}
	})
}

// TestStore_Masking covers RD-AC-5/RD-PARAM-6:
// WithFieldMasks hides masked fields entirely; an erasure tombstone (before=NULL, after=NULL)
// contributes zero entries with no special-case code.
func TestStore_Masking(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	entityID := ids.New()

	// Seed one row that changes both a masked and an unmasked field
	seedFieldHistoryRow(
		t, db, ws, "person", entityID, "human", "user-1", nil, nil,
		map[string]any{"name": "Alice", "ssn": "111-11-1111"},
		map[string]any{"name": "Bob", "ssn": "222-22-2222"},
		nil,
	)

	// Seed an erasure tombstone (before=NULL, after=NULL)
	seedFieldHistoryRow(
		t, db, ws, "person", entityID, "system", "system", nil, nil,
		nil, nil, nil,
	)

	// Use WithFieldMasks to inject a mask that hides "ssn"
	mask := map[string]audithistorydomain.EntityFieldMask{
		"person": {"ssn": {}},
	}
	store := fieldhistory.NewStore(db).WithFieldMasks(mask)
	ctx := context.Background()

	entries, cursor, err := store.List(ctx, ws, "person", entityID, nil, nil, "", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cursor != "" {
		t.Errorf("cursor want empty (exhausted), got %q", cursor)
	}

	// Only "name" entry; "ssn" must never appear; tombstone → 0 entries
	for _, e := range entries {
		if e.Field == "ssn" {
			t.Errorf("masked field 'ssn' must never appear in output, but got entry: %+v", e)
		}
	}
	if len(entries) != 1 {
		t.Errorf("want 1 entry (name only, ssn masked, tombstone zero), got %d: %+v", len(entries), entries)
	}
	if entries[0].Field != "name" {
		t.Errorf("entry.Field = %q, want name", entries[0].Field)
	}
}

// TestStore_Empty covers RD-AC-5's honest-empty requirement:
// a never-seeded entity_id returns (empty slice, "", nil) — never an error.
func TestStore_Empty(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	entityID := ids.New() // not seeded in audit_log

	store := fieldhistory.NewStore(db)
	entries, cursor, err := store.List(context.Background(), ws, "organization", entityID, nil, nil, "", 50)
	if err != nil {
		t.Fatalf("List error want nil, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries for never-seeded entity, got %d", len(entries))
	}
	if entries == nil {
		t.Errorf("want non-nil empty slice, got nil")
	}
	if cursor != "" {
		t.Errorf("cursor want empty, got %q", cursor)
	}
}

// TestStore_Pagination proves cursor pagination: the second page picks up exactly
// where the first left off with no duplicate or skipped entries; the final page returns "".
func TestStore_Pagination(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	entityID := ids.New()
	ctx := context.Background()

	// Seed 3 rows, each changing one distinct field → 3 entries total.
	// Insert from oldest to newest so newest-first order is predictable.
	t1 := time.Now().Add(-3 * time.Second).UTC().Truncate(time.Microsecond)
	t2 := time.Now().Add(-2 * time.Second).UTC().Truncate(time.Microsecond)
	t3 := time.Now().Add(-1 * time.Second).UTC().Truncate(time.Microsecond)

	r1 := seedFieldHistoryRow(t, db, ws, "organization", entityID, "human", "u1", nil, nil,
		map[string]any{"alpha": "A"}, map[string]any{"alpha": "A2"}, &t1)
	r2 := seedFieldHistoryRow(t, db, ws, "organization", entityID, "human", "u1", nil, nil,
		map[string]any{"beta": "B"}, map[string]any{"beta": "B2"}, &t2)
	r3 := seedFieldHistoryRow(t, db, ws, "organization", entityID, "human", "u1", nil, nil,
		map[string]any{"gamma": "C"}, map[string]any{"gamma": "C2"}, &t3)

	// Newest-first order: r3 > r2 > r1
	store := fieldhistory.NewStore(db)

	// Page 1 (limit=2): entries from r3 and r2
	page1, cur1, err := store.List(ctx, ws, "organization", entityID, nil, nil, "", 2)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1: want 2 entries, got %d: %+v", len(page1), page1)
	}
	if cur1 == "" {
		t.Fatal("page1: want non-empty cursor (more pages), got empty")
	}

	// Page 1 should contain r3's entry (gamma) and r2's entry (beta), newest first
	if page1[0].Field != "gamma" || page1[0].ID != r3 {
		t.Errorf("page1[0]: want gamma from r3, got field=%q id=%q", page1[0].Field, page1[0].ID)
	}
	if page1[1].Field != "beta" || page1[1].ID != r2 {
		t.Errorf("page1[1]: want beta from r2, got field=%q id=%q", page1[1].Field, page1[1].ID)
	}

	// Page 2 (using cursor from page1): entry from r1
	page2, cur2, err := store.List(ctx, ws, "organization", entityID, nil, nil, cur1, 2)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("page2: want 1 entry, got %d: %+v", len(page2), page2)
	}
	if cur2 != "" {
		t.Errorf("page2: cursor want empty (exhausted), got %q", cur2)
	}
	if page2[0].Field != "alpha" || page2[0].ID != r1 {
		t.Errorf("page2[0]: want alpha from r1, got field=%q id=%q", page2[0].Field, page2[0].ID)
	}

	// No overlap: all entry ids collected from page1 + page2 are distinct
	seen := map[string]bool{}
	for _, e := range append(page1, page2...) {
		key := e.ID + "/" + e.Field
		if seen[key] {
			t.Errorf("duplicate entry on page boundary: id=%q field=%q", e.ID, e.Field)
		}
		seen[key] = true
	}
	_ = r1
	_ = r2
	_ = r3
}

// TestStore_NewestFirst proves entries are returned newest-first
// even when rows are seeded out of temporal order.
func TestStore_NewestFirst(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	entityID := ids.New()
	ctx := context.Background()

	// Explicit timestamps in a known order
	tOldest := time.Now().Add(-10 * time.Second).UTC().Truncate(time.Microsecond)
	tMiddle := time.Now().Add(-5 * time.Second).UTC().Truncate(time.Microsecond)
	tNewest := time.Now().Add(-1 * time.Second).UTC().Truncate(time.Microsecond)

	// Seed out of temporal order (middle, oldest, newest) to prove the DB sorts them
	seedFieldHistoryRow(t, db, ws, "person", entityID, "human", "u1", nil, nil,
		map[string]any{"x": "m1"}, map[string]any{"x": "m2"}, &tMiddle)
	seedFieldHistoryRow(t, db, ws, "person", entityID, "human", "u1", nil, nil,
		map[string]any{"x": "o1"}, map[string]any{"x": "o2"}, &tOldest)
	seedFieldHistoryRow(t, db, ws, "person", entityID, "human", "u1", nil, nil,
		map[string]any{"x": "n1"}, map[string]any{"x": "n2"}, &tNewest)

	store := fieldhistory.NewStore(db)
	entries, _, err := store.List(ctx, ws, "person", entityID, nil, nil, "", 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}

	// Verify strictly newest-first ordering by changed_at
	for i := 1; i < len(entries); i++ {
		if !entries[i-1].ChangedAt.After(entries[i].ChangedAt) &&
			!entries[i-1].ChangedAt.Equal(entries[i].ChangedAt) {
			t.Errorf("entries not newest-first: entries[%d].ChangedAt=%v entries[%d].ChangedAt=%v",
				i-1, entries[i-1].ChangedAt, i, entries[i].ChangedAt)
		}
	}

	// The first entry (newest) should have old_value="n1"
	if entries[0].OldValue == nil || *entries[0].OldValue != "n1" {
		t.Errorf("first entry (newest): OldValue = %v, want n1", entries[0].OldValue)
	}
	// The last entry (oldest) should have old_value="o1"
	if entries[2].OldValue == nil || *entries[2].OldValue != "o1" {
		t.Errorf("last entry (oldest): OldValue = %v, want o1", entries[2].OldValue)
	}
}
