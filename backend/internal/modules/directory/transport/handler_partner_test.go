//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func openPartnerHandlerTestDB(t *testing.T) *sql.DB {
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

const partnerHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000005"

func seedPartnerHandlerOrg(t *testing.T, db *sql.DB, tag string) string {
	t.Helper()
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t15-h-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, partnerHandlerTestWorkspaceID, "t15-h-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, partnerHandlerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var orgID string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'test', 'human:test') RETURNING id`,
		partnerHandlerTestWorkspaceID, "Org "+tag).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return orgID
}

func withPartnerHandlerPrincipal(r *http.Request, userID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: partnerHandlerTestWorkspaceID, UserID: userID})
	return r.WithContext(ctx)
}

func TestPartnerHandler_UpsertThenGetThenList(t *testing.T) {
	db := openPartnerHandlerTestDB(t)
	orgID := seedPartnerHandlerOrg(t, db, "upsert")
	h := NewPartnerHandler(crmcore.NewPartnerStore(db))

	body := map[string]any{
		"partner_role": "hosting", "cert_status": "applied",
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/organizations/"+orgID+"/partner", bytes.NewReader(b))
	req.SetPathValue("id", orgID)
	req = withPartnerHandlerPrincipal(req, "human:test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, body = %s", w.Code, w.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/partner", nil)
	getReq.SetPathValue("id", orgID)
	getReq = withPartnerHandlerPrincipal(getReq, "human:test")
	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getW.Code, getW.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/partners?partner_role=hosting&cert_status=applied", nil)
	listReq = withPartnerHandlerPrincipal(listReq, "human:test")
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listW.Code, listW.Body.String())
	}
	var page struct {
		Data []crmcore.Partner `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, p := range page.Data {
		if p.OrganizationID == orgID {
			found = true
		}
	}
	if !found {
		t.Fatal("listPartners did not return the upserted partner")
	}
}

func TestPartnerHandler_Get_404WhenNoPartnerRow(t *testing.T) {
	db := openPartnerHandlerTestDB(t)
	orgID := seedPartnerHandlerOrg(t, db, "get-404")
	h := NewPartnerHandler(crmcore.NewPartnerStore(db))

	req := httptest.NewRequest(http.MethodGet, "/organizations/"+orgID+"/partner", nil)
	req.SetPathValue("id", orgID)
	req = withPartnerHandlerPrincipal(req, "human:test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// TestPartnerRBAC_NonAdminDenied proves the denied principal too: a role with
// no "partner" key in its permissions JSONB must get 403 through the real
// RbacMiddleware.
func TestPartnerRBAC_NonAdminDenied(t *testing.T) {
	db := openPartnerHandlerTestDB(t)
	orgID := seedPartnerHandlerOrg(t, db, "rbac")

	const wsID = partnerHandlerTestWorkspaceID
	const adminUserID = "00000000-0000-0000-0010-000000000101"
	const repUserID = "00000000-0000-0000-0010-000000000102"
	if _, err := db.Exec(`INSERT INTO app_user(id,workspace_id,email,display_name) VALUES
		($1,$2,'admin-t15@example.com','Admin'),($3,$2,'rep-t15@example.com','Rep')
		ON CONFLICT DO NOTHING`, adminUserID, wsID, repUserID); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	const adminRoleID = "00000000-0000-0000-0020-000000000101"
	const repRoleID = "00000000-0000-0000-0020-000000000102"
	if _, err := db.Exec(`INSERT INTO role(id,workspace_id,key,is_system,permissions) VALUES
		($1,$2,'t15-admin',true,'{"partner":{"read":{"row_scope":"all"},"update":{"row_scope":"all"}}}'::jsonb),
		($3,$2,'t15-rep',false,'{"organization":{"read":{"row_scope":"all"}}}'::jsonb)
		ON CONFLICT DO NOTHING`, adminRoleID, wsID, repRoleID); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO role_assignment(workspace_id,role_id,user_id) VALUES
		($1,$2,$3),($1,$4,$5)
		ON CONFLICT (role_id,user_id,COALESCE(team_id,'00000000-0000-0000-0000-000000000000'::uuid)) DO NOTHING`,
		wsID, adminRoleID, adminUserID, repRoleID, repUserID); err != nil {
		t.Fatalf("seed role_assignments: %v", err)
	}

	h := NewPartnerHandler(crmcore.NewPartnerStore(db))
	guarded := httpserver.RbacMiddleware(db, httpserver.ObjPartner)(h)

	body := map[string]any{"partner_role": "hosting", "cert_status": "applied", "source": "t", "captured_by": "human:t"}
	b, _ := json.Marshal(body)

	adminReq := httptest.NewRequest(http.MethodPut, "/organizations/"+orgID+"/partner", bytes.NewReader(b))
	adminReq.SetPathValue("id", orgID)
	adminReq = adminReq.WithContext(crmctx.With(adminReq.Context(), crmctx.Principal{TenantID: wsID, UserID: adminUserID}))
	adminW := httptest.NewRecorder()
	guarded.ServeHTTP(adminW, adminReq)
	if adminW.Code == http.StatusForbidden {
		t.Fatalf("admin (has partner:update) must not be denied, got 403: %s", adminW.Body.String())
	}

	repReq := httptest.NewRequest(http.MethodPut, "/organizations/"+orgID+"/partner", bytes.NewReader(b))
	repReq.SetPathValue("id", orgID)
	repReq = repReq.WithContext(crmctx.With(repReq.Context(), crmctx.Principal{TenantID: wsID, UserID: repUserID}))
	repW := httptest.NewRecorder()
	guarded.ServeHTTP(repW, repReq)
	if repW.Code != http.StatusForbidden {
		t.Fatalf("rep (no partner key) must be denied 403, got %d: %s", repW.Code, repW.Body.String())
	}
}
