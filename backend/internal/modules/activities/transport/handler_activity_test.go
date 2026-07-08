//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	actadapters "github.com/gradionhq/margince/backend/internal/modules/activities/adapters"
	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const activityHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000d01"

func openActivityHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func withActivityWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: activityHandlerTestWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

// seedActivityHandlerFixtures seeds a workspace + deal, then a task activity
// linked to that deal via activity_link. ActivityStore.Create does not
// insert activity_link rows (see handler_activity.go's doc comment on why
// POST /activities is unwired), so the link row is inserted directly here,
// matching how store_activity.go's List query reads it.
func seedActivityHandlerFixtures(t *testing.T, db *sql.DB, tag string) (dealID, activityID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t22-activity-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, activityHandlerTestWorkspaceID, "t22-activity-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, activityHandlerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var pipelineID, stageID string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name)
		VALUES (uuidv7(), $1, $2) RETURNING id`, activityHandlerTestWorkspaceID, "P-"+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position)
		VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`, activityHandlerTestWorkspaceID, pipelineID, "S-"+tag).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, source, captured_by)
		VALUES (uuidv7(), $1, $2, $3, $4, 'test', 'human:test') RETURNING id`,
		activityHandlerTestWorkspaceID, "Deal-"+tag, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO activity (id, workspace_id, kind, subject, is_done, source, captured_by)
		VALUES (uuidv7(), $1, 'task', $2, false, 'test', 'human:test') RETURNING id`,
		activityHandlerTestWorkspaceID, "Follow up-"+tag).Scan(&activityID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, deal_id)
		VALUES (uuidv7(), $1, $2, 'deal', $3)`, activityHandlerTestWorkspaceID, activityID, dealID); err != nil {
		t.Fatalf("seed activity_link: %v", err)
	}
	return dealID, activityID
}

func TestActivityHandler_List_FiltersByEntityTypeAndID(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	dealID, activityID := seedActivityHandlerFixtures(t, db, "list")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	req := httptest.NewRequest(http.MethodGet, "/activities?entity_type=deal&entity_id="+dealID, nil)
	req = withActivityWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Data []actdomain.Activity `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != activityID {
		t.Fatalf("expected 1 activity (%s) for deal %s, got %+v", activityID, dealID, page.Data)
	}
}

func TestActivityHandler_Get_ReturnsActivity(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	_, activityID := seedActivityHandlerFixtures(t, db, "get")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	req := withActivityWorkspace(httptest.NewRequest(http.MethodGet, "/activities/"+activityID, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var a actdomain.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.ID != activityID {
		t.Fatalf("got id %s, want %s", a.ID, activityID)
	}
}

func TestActivityHandler_Patch_MarksTaskDone(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	_, activityID := seedActivityHandlerFixtures(t, db, "patch")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	patchBody, _ := json.Marshal(map[string]any{"is_done": true})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPatch, "/activities/"+activityID, bytes.NewReader(patchBody)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var a actdomain.Activity
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !a.IsDone {
		t.Fatal("expected is_done=true after patch")
	}
	if a.DoneAt == nil {
		t.Fatal("expected done_at to be set after marking is_done")
	}
}

func TestActivityHandler_Patch_StaleIfMatch_Returns409VersionSkew(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	_, activityID := seedActivityHandlerFixtures(t, db, "if-match")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	patchBody, _ := json.Marshal(map[string]any{"is_done": true})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPatch, "/activities/"+activityID, bytes.NewReader(patchBody)))
	req.Header.Set("If-Match", "999")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 version_skew, body=%s", w.Code, w.Body.String())
	}
}

func TestActivityHandler_Archive_ExcludesFromDefaultList(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	dealID, activityID := seedActivityHandlerFixtures(t, db, "archive")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	delReq := withActivityWorkspace(httptest.NewRequest(http.MethodDelete, "/activities/"+activityID, nil))
	delW := httptest.NewRecorder()
	h.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body=%s", delW.Code, delW.Body.String())
	}

	listReq := withActivityWorkspace(httptest.NewRequest(http.MethodGet, "/activities?entity_type=deal&entity_id="+dealID, nil))
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	var page struct {
		Data []actdomain.Activity `json:"data"`
	}
	_ = json.Unmarshal(listW.Body.Bytes(), &page)
	for _, a := range page.Data {
		if a.ID == activityID {
			t.Fatal("archived activity must be excluded from the default list")
		}
	}
}

func seedActivityHandlerPerson(t *testing.T, db *sql.DB, tag string) string {
	t.Helper()
	var personID string
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`,
		activityHandlerTestWorkspaceID, "Person-"+tag).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return personID
}

func TestActivityHandler_Post_CreatesThenReplaysIdempotently(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	dealID, _ := seedActivityHandlerFixtures(t, db, "post-idem")
	personID := seedActivityHandlerPerson(t, db, "post-idem")
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]any{
		"kind": "email", "subject": "Re: proposal",
		"source_system": "gmail", "source_id": "post-idem-msg-1",
		"links":       []map[string]any{{"entity_type": "person", "entity_id": personID}, {"entity_type": "deal", "entity_id": dealID}},
		"source":      "email:post-idem-msg-1",
		"captured_by": "agent:capture",
		"raw":         map[string]any{"messageId": "post-idem-msg-1"},
	})

	req1 := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities", bytes.NewReader(body)))
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first POST status = %d, want 201, body=%s", w1.Code, w1.Body.String())
	}
	if w1.Header().Get("Location") == "" {
		t.Fatal("expected a Location header on 201")
	}
	var created actdomain.Activity
	if err := json.Unmarshal(w1.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if len(created.Links) != 2 {
		t.Fatalf("expected 2 links on the created activity, got %d: %+v", len(created.Links), created.Links)
	}
	if created.Raw == nil || created.Raw["messageId"] != "post-idem-msg-1" {
		t.Fatalf("expected raw echoed back, got %+v", created.Raw)
	}

	req2 := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities", bytes.NewReader(body)))
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second POST status = %d, want 200 (idempotent replay), body=%s", w2.Code, w2.Body.String())
	}
	var replayed actdomain.Activity
	if err := json.Unmarshal(w2.Body.Bytes(), &replayed); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if replayed.ID != created.ID {
		t.Fatalf("replay id mismatch: got %s want %s", replayed.ID, created.ID)
	}

	getReq := withActivityWorkspace(httptest.NewRequest(http.MethodGet, "/activities/"+created.ID, nil))
	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET after POST status = %d, body=%s", getW.Code, getW.Body.String())
	}
	var fetched actdomain.Activity
	_ = json.Unmarshal(getW.Body.Bytes(), &fetched)
	if len(fetched.Links) != 2 || fetched.Raw == nil {
		t.Fatalf("GET must return the same links+raw the POST wrote, got links=%d raw=%+v", len(fetched.Links), fetched.Raw)
	}
}

func TestActivityHandler_Post_MissingProvenance_Returns422(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]any{"kind": "note", "body": "no provenance"})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

func TestActivityHandler_Post_TaskFieldOnNonTaskKind_Returns422(t *testing.T) {
	db := openActivityHandlerTestDB(t)
	h := NewActivityHandler(actadapters.NewActivityStore(db))

	body, _ := json.Marshal(map[string]any{
		"kind": "note", "due_at": "2026-08-01T00:00:00Z",
		"source": "ui", "captured_by": "human:test",
	})
	req := withActivityWorkspace(httptest.NewRequest(http.MethodPost, "/activities", bytes.NewReader(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	var problem struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &problem)
	if problem.Code != "field_not_valid_for_kind" {
		t.Fatalf("expected code=field_not_valid_for_kind, got %q (body=%s)", problem.Code, w.Body.String())
	}
}
