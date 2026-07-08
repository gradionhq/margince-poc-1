//go:build integration

package adapters

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

func seedRelinkPerson(t *testing.T, db *sql.DB, wsID, tag string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`, wsID, "P2-"+tag).Scan(&id); err != nil {
		t.Fatalf("seed second person: %v", err)
	}
	return id
}

// scalarCount runs a `SELECT count(*) ...` query and returns the scalar result,
// failing the test on error; it backs the count* helpers below.
func scalarCount(t *testing.T, db *sql.DB, label, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", label, err)
	}
	return n
}

func countActivityRelinkAudit(t *testing.T, db *sql.DB, activityID string) int {
	return scalarCount(t, db, "audit_log",
		`SELECT count(*) FROM audit_log WHERE entity_type='activity' AND entity_id=$1::uuid AND action='activity_relink'`, activityID)
}

func countActivityEventOutbox(t *testing.T, db *sql.DB, activityID string) int {
	return scalarCount(t, db, "event_outbox", `SELECT count(*) FROM event_outbox WHERE entity_id=$1::uuid`, activityID)
}

func countActivityLinksByType(t *testing.T, db *sql.DB, activityID, entityType string) int {
	return scalarCount(t, db, "activity_link",
		`SELECT count(*) FROM activity_link WHERE activity_id=$1::uuid AND entity_type=$2`, activityID, entityType)
}

func TestActivityStore_Relink_AddsFreshLink(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, _ := seedActivityStoreFixtures(t, db, "relink-add")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Relink(context.Background(), created.ID, wsID, "person", personID)
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("relink returned different activity: %s vs %s", got.ID, created.ID)
	}
	if n := countActivityLinksByType(t, db, created.ID, "person"); n != 1 {
		t.Fatalf("want 1 person link, got %d", n)
	}
	if n := countActivityRelinkAudit(t, db, created.ID); n != 1 {
		t.Fatalf("want 1 audit_log relink row, got %d", n)
	}
	if n := countActivityEventOutbox(t, db, created.ID); n != 0 {
		t.Fatalf("want no event_outbox rows, got %d", n)
	}
}

func TestActivityStore_Relink_MovesTypedLinkAndIsIdempotent(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, _ := seedActivityStoreFixtures(t, db, "relink-move")
	otherPersonID := seedRelinkPerson(t, db, wsID, "move")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
		Links: []domain.ActivityLink{{EntityType: "person", EntityID: personID}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	moved, err := s.Relink(context.Background(), created.ID, wsID, "person", otherPersonID)
	if err != nil {
		t.Fatalf("relink move: %v", err)
	}
	if n := countActivityLinksByType(t, db, created.ID, "person"); n != 1 {
		t.Fatalf("want 1 person link after move, got %d", n)
	}
	if n := countActivityRelinkAudit(t, db, created.ID); n != 1 {
		t.Fatalf("want 1 audit_log row after move, got %d", n)
	}

	again, err := s.Relink(context.Background(), moved.ID, wsID, "person", otherPersonID)
	if err != nil {
		t.Fatalf("relink idempotent replay: %v", err)
	}
	if again.ID != moved.ID {
		t.Fatalf("idempotent relink returned different activity: %s vs %s", again.ID, moved.ID)
	}
	if n := countActivityRelinkAudit(t, db, created.ID); n != 1 {
		t.Fatalf("idempotent replay must not add a second audit row, got %d", n)
	}
}

func TestActivityStore_Relink_InvalidEntityTypeRejectedBeforeDBWork(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, _, _ := seedActivityStoreFixtures(t, db, "relink-invalid")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = s.Relink(context.Background(), created.ID, wsID, "team", "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, errs.ErrInvalidLinkEntityType) {
		t.Fatalf("want ErrInvalidLinkEntityType, got %v", err)
	}
	if n := countActivityLinksByType(t, db, created.ID, "team"); n != 0 {
		t.Fatalf("invalid entity_type must not write links, got %d", n)
	}
	if n := countActivityRelinkAudit(t, db, created.ID); n != 0 {
		t.Fatalf("invalid entity_type must not write audit rows, got %d", n)
	}
}

func TestActivityStore_Relink_NonexistentActivity_ReturnsNotFound(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, _ := seedActivityStoreFixtures(t, db, "relink-404")
	s := NewActivityStore(db)

	_, err := s.Relink(context.Background(), "00000000-0000-0000-0000-000000000000", wsID, "person", personID)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestActivityStore_Relink_ArchivedActivity_ReturnsNotFound(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, _ := seedActivityStoreFixtures(t, db, "relink-archived")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.Archive(context.Background(), created.ID, wsID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	_, err = s.Relink(context.Background(), created.ID, wsID, "person", personID)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("want ErrNotFound for an archived activity, got %v", err)
	}
}

func TestActivityStore_Relink_PreservesProvenance_ByteIdentical(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personID, _ := seedActivityStoreFixtures(t, db, "relink-prov")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "email", OccurredAt: time.Now(), Source: "email:msg-1", CapturedBy: "agent:capture",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	beforeSource, beforeCapturedBy := created.Source, created.CapturedBy

	got, err := s.Relink(context.Background(), created.ID, wsID, "person", personID)
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	if got.Source != beforeSource || got.CapturedBy != beforeCapturedBy {
		t.Fatalf("want byte-identical provenance, before=(%s,%s) after=(%s,%s)",
			beforeSource, beforeCapturedBy, got.Source, got.CapturedBy)
	}
}

func TestActivityStore_Relink_SelfHeals_MultipleExistingLinksOfSameType(t *testing.T) {
	db := openActivityStoreTestDB(t)
	wsID, personA, _ := seedActivityStoreFixtures(t, db, "relink-heal")
	personB := seedRelinkPerson(t, db, wsID, "relink-heal-b")
	personC := seedRelinkPerson(t, db, wsID, "relink-heal-c")
	s := NewActivityStore(db)

	created, _, err := s.Create(context.Background(), domain.Activity{
		WorkspaceID: wsID, Kind: "note", OccurredAt: time.Now(), Source: "ui", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Simulate the pre-existing multi-link edge case (only reachable today via
	// Create's own unguarded links[] path): two person-type links on one activity.
	for _, pid := range []string{personA, personB} {
		if _, err := db.Exec(`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, person_id)
			VALUES (uuidv7(), $1, $2, 'person', $3)`, wsID, created.ID, pid); err != nil {
			t.Fatalf("seed duplicate link: %v", err)
		}
	}

	got, err := s.Relink(context.Background(), created.ID, wsID, "person", personC)
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	if n := countActivityLinksByType(t, db, created.ID, "person"); n != 1 {
		t.Fatalf("want self-healing to leave exactly 1 person link, got %d", n)
	}
	// Verify the sole surviving link points at personC.
	personLinks := 0
	for _, l := range got.Links {
		if l.EntityType == "person" {
			personLinks++
			if l.EntityID != personC {
				t.Fatalf("want the sole surviving link to point at C (%s), got %s", personC, l.EntityID)
			}
		}
	}
	if personLinks != 1 {
		t.Fatalf("want exactly 1 person-type link after self-healing, got %d: %+v", personLinks, got.Links)
	}
}
