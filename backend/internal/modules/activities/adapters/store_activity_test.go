//go:build integration

package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	// Registers the postgres driver so sql.Open("postgres", ...) resolves; only the driver's side-effecting init() is used.
	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

func openActivityStoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration` / `make test-it`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedActivityStoreFixtures seeds a workspace + a person + a deal, returning
// their ids for use as activity_link targets.
func seedActivityStoreFixtures(t *testing.T, db *sql.DB, tag string) (wsID, personID, dealID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if err := db.QueryRow(`INSERT INTO workspace (id, name, slug, base_currency)
		VALUES (uuidv7(), $1, $2, 'EUR') RETURNING id::text`,
		"t-activity-store-ws-"+tag, "t-activity-store-ws-slug-"+tag).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, wsID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`, wsID, "P-"+tag).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	var pipelineID, stageID string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		wsID, "P-"+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		wsID, pipelineID, "S-"+tag).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, source, captured_by)
		VALUES (uuidv7(), $1, $2, $3, $4, 'test', 'human:test') RETURNING id`,
		wsID, "Deal-"+tag, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return wsID, personID, dealID
}

func countActivitiesBySource(t *testing.T, db *sql.DB, wsID, sourceSystem, sourceID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM activity WHERE workspace_id=$1 AND source_system=$2 AND source_id=$3`,
		wsID, sourceSystem, sourceID).Scan(&n); err != nil {
		t.Fatalf("count activities: %v", err)
	}
	return n
}

func TestActivityStore_Create_IdempotentReplay_SameSourceKey(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "idem")
	s := NewActivityStore(db)

	base := domain.Activity{
		WorkspaceID: wsID, Kind: "email", OccurredAt: time.Now(),
		SourceSystem: strPtr("gmail"), SourceID: strPtr("msg-1"), Source: "email:msg-1", CapturedBy: "agent:capture",
	}

	first, created1, err := s.Create(context.Background(), base)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if !created1 {
		t.Fatal("expected created=true on the first call")
	}

	second, created2, err := s.Create(context.Background(), base)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if created2 {
		t.Fatal("expected created=false on the idempotent replay")
	}
	if second.ID != first.ID {
		t.Fatalf("replay returned a different id: %s vs %s", second.ID, first.ID)
	}
	if n := countActivitiesBySource(t, db, wsID, "gmail", "msg-1"); n != 1 {
		t.Fatalf("expected exactly 1 row for the source key, got %d", n)
	}
}

func TestActivityStore_Create_NoSourceKey_AlwaysFresh(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "nosrc")
	s := NewActivityStore(db)

	a := domain.Activity{WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test"}
	first, created1, err := s.Create(context.Background(), a)
	if err != nil || !created1 {
		t.Fatalf("first create: created=%v err=%v", created1, err)
	}
	second, created2, err := s.Create(context.Background(), a)
	if err != nil || !created2 {
		t.Fatalf("second create: created=%v err=%v", created2, err)
	}
	if second.ID == first.ID {
		t.Fatal("two manually-logged activities with no source key must not collapse into one row")
	}
}

func TestActivityStore_Create_MultiEntityLinks(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, dealID := seedActivityStoreFixtures(t, db, "links")
	s := NewActivityStore(db)

	a := domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
		Links: []domain.ActivityLink{{EntityType: "person", EntityID: personID}, {EntityType: "deal", EntityID: dealID}},
	}
	created, _, err := s.Create(context.Background(), a)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	rows, err := db.Query(`SELECT entity_type, person_id, organization_id, deal_id FROM activity_link WHERE activity_id=$1`, created.ID)
	if err != nil {
		t.Fatalf("query links: %v", err)
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var entityType string
		var pID, oID, dID sql.NullString
		if err := rows.Scan(&entityType, &pID, &oID, &dID); err != nil {
			t.Fatalf("scan link: %v", err)
		}
		count++
		assertActivityLinkRow(t, entityType, pID, oID, dID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate links: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 activity_link rows, got %d", count)
	}
}

// assertActivityLinkRow checks that exactly the FK column matching entityType
// is populated on an activity_link row, and none of the others.
func assertActivityLinkRow(t *testing.T, entityType string, pID, oID, dID sql.NullString) {
	t.Helper()
	switch entityType {
	case "person":
		if !pID.Valid || oID.Valid || dID.Valid {
			t.Fatalf("person link must have exactly person_id populated, got p=%v o=%v d=%v", pID, oID, dID)
		}
	case "deal":
		if !dID.Valid || pID.Valid || oID.Valid {
			t.Fatalf("deal link must have exactly deal_id populated, got p=%v o=%v d=%v", pID, oID, dID)
		}
	default:
		t.Fatalf("unexpected entity_type %q", entityType)
	}
}

func TestActivityStore_Create_StoresRawOffHotPath(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "raw")
	s := NewActivityStore(db)

	a := domain.Activity{
		WorkspaceID: wsID, Kind: "email", OccurredAt: time.Now(), Source: "email:x", CapturedBy: "agent:capture",
		Raw: map[string]any{"messageId": "abc123", "headers": map[string]any{"from": "a@b.com"}},
	}
	created, _, err := s.Create(context.Background(), a)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var rawJSON []byte
	if err := db.QueryRow(`SELECT raw FROM activity WHERE id=$1`, created.ID).Scan(&rawJSON); err != nil {
		t.Fatalf("select raw: %v", err)
	}
	if rawJSON == nil {
		t.Fatal("expected raw to be persisted, got NULL")
	}
}

func TestActivityStore_Create_MissingProvenance_RejectedBeforeInsert(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "prov")
	s := NewActivityStore(db)

	_, _, err := s.Create(context.Background(), domain.Activity{WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "", CapturedBy: ""})
	if err == nil {
		t.Fatal("expected an error for missing provenance")
	}
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("expected ErrNullProvenance, got %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM activity WHERE workspace_id=$1`, wsID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no row inserted, got %d", n)
	}
}

func TestActivityStore_Create_TaskFieldOnNonTaskKind_Rejected(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "taskfield")
	s := NewActivityStore(db)

	due := time.Now().Add(24 * time.Hour)
	_, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), DueAt: &due, Source: "ui", CapturedBy: "human:test",
	})
	if !errors.Is(err, errs.ErrFieldNotValidForKind) {
		t.Fatalf("expected ErrFieldNotValidForKind, got %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM activity WHERE workspace_id=$1`, wsID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no row inserted on validation failure, got %d", n)
	}
}

func TestActivityStore_Create_TaskKind_AllowsTaskFields(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "taskok")
	s := NewActivityStore(db)

	due := time.Now().Add(24 * time.Hour)
	_, created, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "task", OccurredAt: time.Now(), DueAt: &due, Source: "ui", CapturedBy: "human:test",
	})
	if err != nil || !created {
		t.Fatalf("expected a task activity with due_at to succeed: created=%v err=%v", created, err)
	}
}

func strPtr(s string) *string { return &s }

func TestActivityStore_Get_ReturnsLinksAndRaw(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, dealID := seedActivityStoreFixtures(t, db, "getlinks")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
		Raw:   map[string]any{"k": "v"},
		Links: []domain.ActivityLink{{EntityType: "person", EntityID: personID}, {EntityType: "deal", EntityID: dealID}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Links) != 2 {
		t.Fatalf("expected 2 links on Get, got %d: %+v", len(got.Links), got.Links)
	}
	if got.Raw == nil || got.Raw["k"] != "v" {
		t.Fatalf("expected raw={k:v} on Get, got %+v", got.Raw)
	}
	linkedTypes := map[string]bool{}
	for _, l := range got.Links {
		linkedTypes[l.EntityType] = true
		if l.ActivityID != created.ID {
			t.Fatalf("link activity_id mismatch: got %s want %s", l.ActivityID, created.ID)
		}
	}
	if !linkedTypes["person"] || !linkedTypes["deal"] {
		t.Fatalf("expected person+deal links, got %+v", got.Links)
	}
}

func TestActivityStore_Get_NotFound_404(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "get404")
	s := NewActivityStore(db)
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000000", wsID)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestActivityStore_List_NeverSelectsRaw(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "listraw")
	s := NewActivityStore(db)

	if _, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
		Raw: map[string]any{"sensitive": "payload"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, _, err := s.List(context.Background(), wsID, "", "", "", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one activity")
	}
	for _, a := range items {
		if a.Raw != nil {
			t.Fatalf("List must never populate Raw (hot-path exclusion, ACT-AC-2), got %+v", a.Raw)
		}
	}
}

func TestActivityStore_List_LinksNeverNil(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "listlinksnil")
	s := NewActivityStore(db)

	if _, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, _, err := s.List(context.Background(), wsID, "", "", "", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one activity")
	}
	for _, a := range items {
		if a.Links == nil {
			t.Fatalf("List must never leave Links nil (JSON null vs [] wire shape, crm.yaml Activity.links is a non-nullable array), got nil for activity %s", a.ID)
		}
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatalf("marshal activity %s: %v", a.ID, err)
		}
		if strings.Contains(string(b), `"links":null`) {
			t.Fatalf("List activity %s marshaled with links:null, want links:[]: %s", a.ID, b)
		}
		if !strings.Contains(string(b), `"links":[]`) {
			t.Fatalf("List activity %s expected empty links array in JSON, got: %s", a.ID, b)
		}
	}
}

func TestActivityStore_Update_TaskFieldOnNonTaskKind_Rejected(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "updtaskfield")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = s.Update(context.Background(), created.ID, wsID, map[string]any{"is_done": true}, 0)
	if !errors.Is(err, errs.ErrFieldNotValidForKind) {
		t.Fatalf("expected ErrFieldNotValidForKind updating is_done on a note, got %v", err)
	}
}

func TestActivityStore_Update_TaskKind_AllowsIsDone(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "updtaskok")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "task", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	updated, err := s.Update(context.Background(), created.ID, wsID, map[string]any{"is_done": true}, 0)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.IsDone {
		t.Fatal("expected is_done=true after update")
	}
}

func TestActivityStore_ProvenanceRatio_MixedFixture(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "ratio")
	s := NewActivityStore(db)

	// 2 agent-captured, 3 human-captured.
	capturedBys := []string{"agent:capture", "agent:capture", "human:u1", "human:u2", "human:u1"}
	for i, cb := range capturedBys {
		if _, _, err := s.Create(context.Background(), domain.Activity{
			WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(),
			Source: "test", CapturedBy: cb, Subject: strPtr(fmt.Sprintf("n-%d", i)),
		}); err != nil {
			t.Fatalf("seed activity %d: %v", i, err)
		}
	}

	agent, human, err := s.ProvenanceRatio(context.Background(), wsID)
	if err != nil {
		t.Fatalf("ProvenanceRatio: %v", err)
	}
	if agent != 2 || human != 3 {
		t.Fatalf("expected agent=2 human=3, got agent=%d human=%d", agent, human)
	}
}
