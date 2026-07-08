//go:build integration

package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// testWorkspaceID is the fixed workspace every handler_person_test.go case
// seeds, sets RLS for, and stamps onto its request principal.
const testWorkspaceID = "00000000-0000-0000-0000-000000000001"

func withWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: testWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

// setRLS's wsID parameter varies across this file's call sites: most cases
// share testWorkspaceID, but handler_person_strength_test.go's
// TestPersonHandler_List_SortStrength_EmptyWorkspace uses a second, distinct
// workspace ID so it can prove the zero-people case on a genuinely empty
// workspace. Kept as a real parameter (not simplified to a no-arg form) to
// preserve the exact shape of modules/directory's sibling helper this
// duplicates (see seedWorkspace's doc below).
func setRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		"SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
	}
}

// seedWorkspace duplicates modules/directory's helpers_shared_test.go helper of
// the same name: this test moved from package crmcore (modules/directory) to
// package transport (modules/people/transport) in the 1c restructure
// (task-3-brief.md) and the two packages can no longer share a _test.go file,
// so the 9-line helper is copied rather than exported solely for this — same
// class of directory-move-forced duplication as httpserver's
// keyStatus/statusRecorder (see internal/platform/httpserver/middleware.go).
// Its wsID parameter varies across call sites for the same reason as
// setRLS above.
func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'e03test',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "e03-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

func personHandlerForTest(db *sql.DB, store *people.PersonStore) *PersonHandler {
	return NewPersonHandler(store, relationships.NewRelationshipStore(db), deals.NewDealStore(db), activities.NewActivityStore(db), &crmapprovals.DBVerifier{DB: db})
}

func TestPersonHandler_CreateAndGet(t *testing.T) {
	db := openTestDB(t)
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	const wsID = "00000000-0000-0000-0000-000000000001"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	// POST /people
	body := map[string]any{
		"full_name":   "Alice Test",
		"source":      "test",
		"captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/people", bytes.NewReader(b))
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /people: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var created people.Person
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("created.ID empty")
	}
	if created.FullName != "Alice Test" {
		t.Fatalf("got FullName=%q", created.FullName)
	}

	// GET /people/{id}
	req2 := httptest.NewRequest(http.MethodGet, "/people/"+created.ID, nil)
	req2 = withWorkspace(req2)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("GET /people/{id}: want 200, got %d", w2.Code)
	}
}

func TestPersonHandler_List(t *testing.T) {
	db := openTestDB(t)
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	const wsID = "00000000-0000-0000-0000-000000000001"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	// Seed one person directly via store
	p := people.NewPerson("Bob Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})
	if _, err := store.Create(ctx, p, nil); err != nil {
		t.Fatal("seed:", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/people", nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /people: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["data"] == nil {
		t.Fatal("expected data key in response")
	}
}

func TestPersonHandler_UpdateAndArchive(t *testing.T) {
	db := openTestDB(t)
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	const wsID = "00000000-0000-0000-0000-000000000001"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	// Create
	p := people.NewPerson("Charlie Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})
	created, err := store.Create(ctx, p, nil)
	if err != nil {
		t.Fatal("create:", err)
	}

	// PATCH
	upd := map[string]any{"full_name": "Charlie Updated"}
	b, _ := json.Marshal(upd)
	req := httptest.NewRequest(http.MethodPatch, "/people/"+created.ID, bytes.NewReader(b))
	req.Header.Set("If-Match", "1")
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PATCH /people/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// DELETE
	req2 := httptest.NewRequest(http.MethodDelete, "/people/"+created.ID, nil)
	req2 = withWorkspace(req2)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("DELETE /people/{id}: want 200, got %d", w2.Code)
	}
}

// T06 UAT step 2 / crm.yaml getPerson's "Fetchable by id even when archived"
// description: after a merge archives the loser, GET /people/{loserID} must
// still 200, not 404, mirroring OrgStore.GetAny's contract.
func TestPersonHandler_GetArchivedAfterMerge(t *testing.T) {
	db := openTestDB(t)
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	const wsID = "00000000-0000-0000-0000-000000000001"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	loser := people.NewPerson("Loser Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	loser.WorkspaceID = wsID
	createdLoser, err := store.Create(ctx, loser, nil)
	if err != nil {
		t.Fatal("create loser:", err)
	}
	target := people.NewPerson("Target Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	target.WorkspaceID = wsID
	createdTarget, err := store.Create(ctx, target, nil)
	if err != nil {
		t.Fatal("create target:", err)
	}
	if _, err := store.Merge(ctx, createdLoser.ID, createdTarget.ID, wsID); err != nil {
		t.Fatal("merge:", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/people/"+createdLoser.ID, nil)
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /people/{archived loser id}: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["archived_at"] == nil {
		t.Fatalf("expected archived_at set on merged loser, got %v", body)
	}
	if body["merged_into_id"] != createdTarget.ID {
		t.Fatalf("expected merged_into_id=%s, got %v", createdTarget.ID, body["merged_into_id"])
	}
}

func TestPersonHandler_VersionSkew(t *testing.T) {
	db := openTestDB(t)
	store := people.NewPersonStore(db)
	h := personHandlerForTest(db, store)

	const wsID = "00000000-0000-0000-0000-000000000001"
	seedWorkspace(t, db, wsID)
	setRLS(t, db, wsID)

	p := people.NewPerson("Dave Test", prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = wsID
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})
	created, err := store.Create(ctx, p, nil)
	if err != nil {
		t.Fatal("create:", err)
	}

	// PATCH with wrong version
	upd := map[string]any{"full_name": "Dave Conflict"}
	b, _ := json.Marshal(upd)
	req := httptest.NewRequest(http.MethodPatch, "/people/"+created.ID, bytes.NewReader(b))
	req.Header.Set("If-Match", "99")
	req = withWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 on version skew, got %d", w.Code)
	}
}
