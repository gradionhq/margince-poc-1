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

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver

	actadapters "github.com/gradionhq/margince/backend/internal/modules/activities/adapters"
	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
)

// seedRelinkHandlerFixtures seeds a workspace, two people (in a single
// multi-row insert), and one bare activity.
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
	rows, err := db.Query(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test'), (uuidv7(), $1, $3, 'test', 'human:test')
		RETURNING id`, activityHandlerTestWorkspaceID, "PA-"+tag, "PB-"+tag)
	if err != nil {
		t.Fatalf("seed people: %v", err)
	}
	defer rows.Close()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan person id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("seed people rows: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 seeded person ids, got %d", len(ids))
	}
	personA, personB = ids[0], ids[1]
	if err := db.QueryRow(`INSERT INTO activity (id, workspace_id, kind, subject, is_done, source, captured_by)
		VALUES (uuidv7(), $1, 'note', $2, false, 'ui', 'human:test') RETURNING id`,
		activityHandlerTestWorkspaceID, "Relink target-"+tag).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	return personA, personB, activityID
}

// decodeRelinkOK asserts w is a 200 response and decodes its body into an
// Activity, failing the test on either a non-200 status or an undecodable
// body — the boilerplate every relink-success test below shares.
func decodeRelinkOK(t *testing.T, w *httptest.ResponseRecorder) actdomain.Activity {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var a actdomain.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return a
}

func TestActivityHandler_Relink_AddsLink(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	personA, _, activityID := seedRelinkHandlerFixtures(t, db, "add")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]string{"entity_type": "person", "entity_id": personA})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities/"+activityID+"/relink", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	a := decodeRelinkOK(t, w)
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

	a := decodeRelinkOK(t, w)
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

	a := decodeRelinkOK(t, w)
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
