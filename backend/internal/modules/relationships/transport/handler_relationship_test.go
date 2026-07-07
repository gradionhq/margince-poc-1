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

	reladapters "github.com/gradionhq/margince/backend/internal/modules/relationships/adapters"
	reldomain "github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
	platformauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const relHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000c01"

func openRelHandlerTestDB(t *testing.T) *sql.DB {
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

func withRelWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: relHandlerTestWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func withRelWorkspaceUser(r *http.Request, userID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: relHandlerTestWorkspaceID, UserID: userID})
	return r.WithContext(ctx)
}

func seedRelHandlerFixtures(t *testing.T, db *sql.DB, tag string) (personID, orgID string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t08-handler-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, relHandlerTestWorkspaceID, "t08-handler-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, relHandlerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO person (id, workspace_id, full_name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`, relHandlerTestWorkspaceID, "Person-"+tag).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, classification, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'prospect', 'test', 'human:test') RETURNING id`, relHandlerTestWorkspaceID, "Org-"+tag).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return personID, orgID
}

func TestRelationshipHandler_CreateEmployment_Returns201_ThenListedByOrgAndKind(t *testing.T) {
	db := openRelHandlerTestDB(t)
	personID, orgID := seedRelHandlerFixtures(t, db, "create")
	h := NewRelationshipHandler(reladapters.NewRelationshipStore(db))

	body := map[string]any{
		"kind": "employment", "person_id": personID, "organization_id": orgID,
		"role": "vp_engineering", "is_current_primary": true,
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(b))
	req = withRelWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc == "" {
		t.Fatal("expected Location header on 201")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/relationships?organization_id="+orgID+"&kind=employment", nil)
	listReq = withRelWorkspace(listReq)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listW.Code, listW.Body.String())
	}
	var page struct {
		Data []reldomain.Relationship `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].PersonID == nil || *page.Data[0].PersonID != personID {
		t.Fatalf("expected 1 employment row for org %s, got %+v", orgID, page.Data)
	}
}

func TestRelationshipHandler_CreatePartnerKind_Returns422(t *testing.T) {
	db := openRelHandlerTestDB(t)
	_, orgA := seedRelHandlerFixtures(t, db, "partner-a")
	_, orgB := seedRelHandlerFixtures(t, db, "partner-b")
	h := NewRelationshipHandler(reladapters.NewRelationshipStore(db))

	body := map[string]any{
		"kind": "partner_of", "organization_id": orgA, "counterparty_org_id": orgB,
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(b))
	req = withRelWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 for a partner-kind create (T15/A41 owns that surface), body=%s", w.Code, w.Body.String())
	}
}

func TestRelationshipHandler_Update_StaleIfMatch_Returns409VersionSkew(t *testing.T) {
	db := openRelHandlerTestDB(t)
	personID, orgID := seedRelHandlerFixtures(t, db, "update")
	h := NewRelationshipHandler(reladapters.NewRelationshipStore(db))

	createBody, _ := json.Marshal(map[string]any{
		"kind": "employment", "person_id": personID, "organization_id": orgID,
		"role": "cto", "source": "test", "captured_by": "human:test",
	})
	createReq := withRelWorkspace(httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(createBody)))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	var created reldomain.Relationship
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	patchBody, _ := json.Marshal(map[string]any{"role": "ceo"})
	patchReq := withRelWorkspace(httptest.NewRequest(http.MethodPatch, "/relationships/"+created.ID, bytes.NewReader(patchBody)))
	patchReq.Header.Set("If-Match", "999")
	patchW := httptest.NewRecorder()
	h.ServeHTTP(patchW, patchReq)

	if patchW.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 version_skew, body=%s", patchW.Code, patchW.Body.String())
	}
}

func TestRelationshipHandler_Archive_ExcludesFromDefaultList(t *testing.T) {
	db := openRelHandlerTestDB(t)
	personID, orgID := seedRelHandlerFixtures(t, db, "archive")
	h := NewRelationshipHandler(reladapters.NewRelationshipStore(db))

	createBody, _ := json.Marshal(map[string]any{
		"kind": "employment", "person_id": personID, "organization_id": orgID,
		"source": "test", "captured_by": "human:test",
	})
	createReq := withRelWorkspace(httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(createBody)))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	var created reldomain.Relationship
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	delReq := withRelWorkspace(httptest.NewRequest(http.MethodDelete, "/relationships/"+created.ID, nil))
	delW := httptest.NewRecorder()
	h.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body=%s", delW.Code, delW.Body.String())
	}

	listReq := withRelWorkspace(httptest.NewRequest(http.MethodGet, "/relationships?person_id="+personID, nil))
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	var page struct {
		Data []reldomain.Relationship `json:"data"`
	}
	_ = json.Unmarshal(listW.Body.Bytes(), &page)
	for _, r := range page.Data {
		if r.ID == created.ID {
			t.Fatal("archived relationship must be excluded from the default list")
		}
	}

	archivedListReq := withRelWorkspace(httptest.NewRequest(http.MethodGet, "/relationships?person_id="+personID+"&include_archived=true", nil))
	archivedListW := httptest.NewRecorder()
	h.ServeHTTP(archivedListW, archivedListReq)
	var archivedPage struct {
		Data []reldomain.Relationship `json:"data"`
	}
	_ = json.Unmarshal(archivedListW.Body.Bytes(), &archivedPage)
	found := false
	for _, r := range archivedPage.Data {
		if r.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected archived relationship in include_archived=true list")
	}
}

func TestRelationshipHandler_RBACObject_WiresRelationshipKey(t *testing.T) {
	db := openRelHandlerTestDB(t)
	personID, orgID := seedRelHandlerFixtures(t, db, "rbac")
	h := NewRelationshipHandler(reladapters.NewRelationshipStore(db))
	guarded := platformauth.RbacMiddleware(db, platformauth.ObjRelationship)(h)

	const (
		allowedUserID = "00000000-0000-0000-0010-000000000c11"
		deniedUserID  = "00000000-0000-0000-0010-000000000c12"
		allowedRoleID = "00000000-0000-0000-0020-000000000c11"
		deniedRoleID  = "00000000-0000-0000-0020-000000000c12"
	)
	if _, err := db.Exec(`INSERT INTO app_user(id,workspace_id,email,display_name) VALUES
		($1,$2,'allowed-t08@example.com','Allowed'),($3,$2,'denied-t08@example.com','Denied')
		ON CONFLICT DO NOTHING`, allowedUserID, relHandlerTestWorkspaceID, deniedUserID); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role(id,workspace_id,key,is_system,permissions) VALUES
		($1,$2,'t08-allowed',true,'{"relationship":{"create":{"row_scope":"all"}}}'::jsonb),
		($3,$2,'t08-denied',false,'{"organization":{"read":{"row_scope":"all"}}}'::jsonb)
		ON CONFLICT DO NOTHING`, allowedRoleID, relHandlerTestWorkspaceID, deniedRoleID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role_assignment(workspace_id,role_id,user_id) VALUES
		($1,$2,$3),($1,$4,$5)
		ON CONFLICT (role_id,user_id,COALESCE(team_id,'00000000-0000-0000-0000-000000000000'::uuid)) DO NOTHING`,
		relHandlerTestWorkspaceID, allowedRoleID, allowedUserID, deniedRoleID, deniedUserID); err != nil {
		t.Fatalf("seed role assignments: %v", err)
	}

	body := map[string]any{
		"kind": "employment", "person_id": personID, "organization_id": orgID,
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)

	allowedReq := httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(b))
	allowedReq = withRelWorkspaceUser(allowedReq, allowedUserID)
	allowedW := httptest.NewRecorder()
	guarded.ServeHTTP(allowedW, allowedReq)
	if allowedW.Code != http.StatusCreated {
		t.Fatalf("allowed request status = %d, body=%s", allowedW.Code, allowedW.Body.String())
	}

	deniedReq := httptest.NewRequest(http.MethodPost, "/relationships", bytes.NewReader(b))
	deniedReq = withRelWorkspaceUser(deniedReq, deniedUserID)
	deniedW := httptest.NewRecorder()
	guarded.ServeHTTP(deniedW, deniedReq)
	if deniedW.Code != http.StatusForbidden {
		t.Fatalf("denied request status = %d, want 403, body=%s", deniedW.Code, deniedW.Body.String())
	}
}
