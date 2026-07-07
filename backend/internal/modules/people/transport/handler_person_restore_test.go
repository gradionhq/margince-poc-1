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
)

const personRestoreHandlerWS = "00000000-0000-0000-0000-000000000033"

func openPersonHandlerRestoreTestDB(t *testing.T) *sql.DB {
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

func withRestoreWorkspace(r *http.Request, wsID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestPersonHandler_Restore_ArchivedPerson(t *testing.T) {
	db := openPersonHandlerRestoreTestDB(t)
	store := people.NewPersonStore(db)
	h := NewPersonHandler(store, relationships.NewRelationshipStore(db), deals.NewDealStore(db), activities.NewActivityStore(db), &crmapprovals.DBVerifier{DB: db})

	seedWorkspace(t, db, personRestoreHandlerWS)
	setRLS(t, db, personRestoreHandlerWS)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: personRestoreHandlerWS, UserID: "human:test"})
	created, err := store.Create(ctx, people.Person{
		WorkspaceID: personRestoreHandlerWS,
		FullName:    "Handler Restore",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	if _, err := store.Archive(ctx, created.ID, personRestoreHandlerWS); err != nil {
		t.Fatalf("archive person: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/people/"+created.ID+"/restore", bytes.NewReader(nil))
	req = withRestoreWorkspace(req, personRestoreHandlerWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /people/{id}/restore status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var restored people.Person
	if err := json.NewDecoder(w.Body).Decode(&restored); err != nil {
		t.Fatalf("decode restore response: %v", err)
	}
	if restored.ArchivedAt != nil {
		t.Fatal("restore response returned archived_at != null")
	}
}

func TestPersonHandler_Restore_RefusesLiveRecord(t *testing.T) {
	db := openPersonHandlerRestoreTestDB(t)
	store := people.NewPersonStore(db)
	h := NewPersonHandler(store, relationships.NewRelationshipStore(db), deals.NewDealStore(db), activities.NewActivityStore(db), &crmapprovals.DBVerifier{DB: db})

	seedWorkspace(t, db, personRestoreHandlerWS)
	setRLS(t, db, personRestoreHandlerWS)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: personRestoreHandlerWS, UserID: "human:test"})
	created, err := store.Create(ctx, people.Person{
		WorkspaceID: personRestoreHandlerWS,
		FullName:    "Live Handler Restore",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create person: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/people/"+created.ID+"/restore", bytes.NewReader(nil))
	req = withRestoreWorkspace(req, personRestoreHandlerWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /people/{id}/restore status=%d want 422, body=%s", w.Code, w.Body.String())
	}
	var problem map[string]any
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["code"] != "validation_error" {
		t.Fatalf("problem code=%v want validation_error", problem["code"])
	}
	details, ok := problem["details"].(map[string]any)
	if !ok {
		t.Fatalf("problem details missing: %v", problem)
	}
	errs, ok := details["errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("problem details.errors=%v want one entry", details["errors"])
	}
	first := errs[0].(map[string]any)
	if first["field"] != "archived_at" {
		t.Fatalf("field=%v want archived_at", first["field"])
	}
	if first["code"] != "not_archived" {
		t.Fatalf("code=%v want not_archived", first["code"])
	}
}

func TestPersonHandler_Restore_RefusesMergedRecord(t *testing.T) {
	db := openPersonHandlerRestoreTestDB(t)
	store := people.NewPersonStore(db)
	h := NewPersonHandler(store, relationships.NewRelationshipStore(db), deals.NewDealStore(db), activities.NewActivityStore(db), &crmapprovals.DBVerifier{DB: db})

	seedWorkspace(t, db, personRestoreHandlerWS)
	setRLS(t, db, personRestoreHandlerWS)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: personRestoreHandlerWS, UserID: "human:test"})
	survivor, err := store.Create(ctx, people.Person{
		WorkspaceID: personRestoreHandlerWS,
		FullName:    "Survivor Person",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create survivor person: %v", err)
	}
	merged, err := store.Create(ctx, people.Person{
		WorkspaceID: personRestoreHandlerWS,
		FullName:    "Merged Handler Person",
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("create merged person: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE person
		 SET archived_at = now(), merged_into_id = $1::uuid
		 WHERE id = $2::uuid AND workspace_id = $3::uuid`,
		survivor.ID, merged.ID, personRestoreHandlerWS,
	); err != nil {
		t.Fatalf("seed merged person state: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/people/"+merged.ID+"/restore", bytes.NewReader(nil))
	req = withRestoreWorkspace(req, personRestoreHandlerWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST /people/{id}/restore status=%d want 422, body=%s", w.Code, w.Body.String())
	}
	var problem map[string]any
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["code"] != "validation_error" {
		t.Fatalf("problem code=%v want validation_error", problem["code"])
	}
	details, ok := problem["details"].(map[string]any)
	if !ok {
		t.Fatalf("problem details missing: %v", problem)
	}
	errList, ok := details["errors"].([]any)
	if !ok || len(errList) != 1 {
		t.Fatalf("problem details.errors=%v want one entry", details["errors"])
	}
	first := errList[0].(map[string]any)
	if first["field"] != "merged_into_id" {
		t.Fatalf("field=%v want merged_into_id", first["field"])
	}
	if first["code"] != "merged_record" {
		t.Fatalf("code=%v want merged_record", first["code"])
	}
}
