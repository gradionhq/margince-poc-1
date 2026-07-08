//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"

	actadapters "github.com/gradionhq/margince/backend/internal/modules/activities/adapters"
	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
)

// insertRelinkPerson inserts a single test person tagged with the given suffix
// and returns its id; it backs seedRelinkHandlerFixtures' two-person seed below.
func insertRelinkPerson(t *testing.T, db *sql.DB, wsID, tag string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`, wsID, tag).Scan(&id); err != nil {
		t.Fatalf("seed person %s: %v", tag, err)
	}
	return id
}

// seedRelinkHandlerFixtures seeds a workspace, two people, and one bare activity.
func seedRelinkHandlerFixtures(t *testing.T, db *sql.DB, tag string) (personA, personB, activityID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t-at-t05-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, activityHandlerTestWorkspaceID, "t-at-t05-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, activityHandlerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	personA = insertRelinkPerson(t, db, activityHandlerTestWorkspaceID, "PA-"+tag)
	personB = insertRelinkPerson(t, db, activityHandlerTestWorkspaceID, "PB-"+tag)
	if err := db.QueryRow(`INSERT INTO activity (id, workspace_id, kind, subject, is_done, source, captured_by)
		VALUES (uuidv7(), $1, 'note', $2, false, 'ui', 'human:test') RETURNING id`,
		activityHandlerTestWorkspaceID, "Relink target-"+tag).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	return personA, personB, activityID
}

func TestActivityHandler_Relink_AddsLink(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	personA, _, activityID := seedRelinkHandlerFixtures(t, db, "add")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]string{"entity_type": "person", "entity_id": personA})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var a actdomain.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(a.Links) != 1 || a.Links[0].EntityID != personA {
		t.Fatalf("expected one link to %s, got %+v", personA, a.Links)
	}
}

func TestActivityHandler_Relink_Replay_IsNoOp(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	personA, _, activityID := seedRelinkHandlerFixtures(t, db, "replay")
	h := NewActivityHandler(actadapters.NewActivityStore(db))
	body, _ := json.Marshal(map[string]string{"entity_type": "person", "entity_id": personA})

	h.ServeHTTP(httptest.NewRecorder(), withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(body))))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(body))))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var a actdomain.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(a.Links) != 1 || a.Links[0].EntityID != personA {
		t.Fatalf("expected the same single link to %s after replay, got %+v", personA, a.Links)
	}
}

func TestActivityHandler_Relink_Move_ReplacesTarget(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	personA, personB, activityID := seedRelinkHandlerFixtures(t, db, "move")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	firstBody, _ := json.Marshal(map[string]string{"entity_type": "person", "entity_id": personA})
	h.ServeHTTP(httptest.NewRecorder(), withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(firstBody))))

	secondBody, _ := json.Marshal(map[string]string{"entity_type": "person", "entity_id": personB})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(secondBody))))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var a actdomain.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	personLinks := 0
	for _, l := range a.Links {
		if l.EntityType == "person" {
			personLinks++
			if l.EntityID != personB {
				t.Fatalf("expected the surviving link to point at B (%s), got %s", personB, l.EntityID)
			}
		}
	}
	if personLinks != 1 {
		t.Fatalf("expected exactly 1 person link after the move, got %d: %+v", personLinks, a.Links)
	}
}

func TestActivityHandler_Relink_InvalidEntityType_Returns422(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	_, _, activityID := seedRelinkHandlerFixtures(t, db, "badtype")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]string{"entity_type": "lead", "entity_id": "00000000-0000-0000-0000-000000000000"})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	var problem struct {
		Details struct {
			Errors []struct {
				Field string `json:"field"`
				Code  string `json:"code"`
			} `json:"errors"`
		} `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(problem.Details.Errors) != 1 || problem.Details.Errors[0].Field != "entity_type" || problem.Details.Errors[0].Code != "invalid_link_entity_type" {
		t.Fatalf("expected entity_type/invalid_link_entity_type field error, got %+v", problem.Details.Errors)
	}
}

func TestActivityHandler_Relink_NonexistentActivity_Returns404(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	personA, _, _ := seedRelinkHandlerFixtures(t, db, "404")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]string{"entity_type": "person", "entity_id": personA})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/00000000-0000-0000-0000-000000000000/relink", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", w.Code, w.Body.String())
	}
}
