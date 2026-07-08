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

func countActivityRelinkAudit(t *testing.T, db *sql.DB, activityID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_type='activity' AND entity_id=$1::uuid AND action='activity_relink'`,
		activityID).Scan(&n); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	return n
}

func countActivityEventOutbox(t *testing.T, db *sql.DB, activityID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE entity_id=$1::uuid`, activityID).Scan(&n); err != nil {
		t.Fatalf("count event_outbox: %v", err)
	}
	return n
}

func countActivityLinksByType(t *testing.T, db *sql.DB, activityID, entityType string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM activity_link WHERE activity_id=$1::uuid AND entity_type=$2`,
		activityID, entityType).Scan(&n); err != nil {
		t.Fatalf("count activity_link: %v", err)
	}
	return n
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
