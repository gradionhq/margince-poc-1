//go:build integration

package transport

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// testDBURL duplicates cmd/api/tracing_integration_test.go's helper of the same
// name: this test moved from package main (cmd/api) to package transport (1c
// restructure, task-3-brief.md) and the two packages can no longer share a
// _test.go file, so the 5-line helper is copied rather than exported solely for
// this — same class of directory-move-forced duplication as httpserver's
// keyStatus/statusRecorder (see internal/platform/httpserver/middleware.go).
func testDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
}

// These tests exercise the REAL exported handlers (HandleListRoles,
// HandleListMembers, HandleAssignRole, HandleRevokeRole) + the REAL
// RequireManageMembers gate, routed through a real ServeMux so r.PathValue works.
// They need a Postgres-backed DB (RLS + partial-unique index + JOINs are real-DB
// behaviors a memstore can't model). Run with:
//
//	make infra-up && make test-db-up
//	go test -tags=integration ./internal/modules/identity/transport/ -run TestMembers -v
//
// injectPrincipal mirrors the dev/test header path of routes.go's workspaceWrap
// closure (X-Workspace-ID / X-User-ID → crmctx.Principal) without the session
// middleware, so httptest can drive the handlers exactly like a domain route.
func injectPrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsID := r.Header.Get("X-Workspace-ID")
		userID := r.Header.Get("X-User-ID")
		if wsID != "" {
			ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: wsID, UserID: userID})
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// membersTestMux wires the four routes the same way routes.go does — through the
// real RequireManageMembers gate and a real ServeMux (so {user_id}/{role_key}
// wildcards populate r.PathValue).
func membersTestMux(db *sql.DB) *http.ServeMux {
	wrap := func(h http.Handler) http.Handler { return injectPrincipal(RequireManageMembers(db, h)) }
	mux := http.NewServeMux()
	mux.Handle("GET /roles", wrap(HandleListRoles(db)))
	mux.Handle("GET /members", wrap(HandleListMembers(db)))
	mux.Handle("POST /members/{user_id}/roles", wrap(HandleAssignRole(db)))
	mux.Handle("DELETE /members/{user_id}/roles/{role_key}", wrap(HandleRevokeRole(db)))
	return mux
}

type membersFixture struct {
	db          *sql.DB
	mux         *http.ServeMux
	ws          string
	otherWS     string
	adminUser   string
	repUser     string
	otherUser   string // member of otherWS
	adminRole   string
	managerRole string
	repRole     string
}

func mustExec(ctx context.Context, t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func setupMembersFixture(ctx context.Context, t *testing.T) *membersFixture {
	t.Helper()
	db, err := sql.Open("postgres", testDBURL())
	if err != nil {
		t.Fatal(err)
	}
	f := &membersFixture{
		db:          db,
		ws:          ids.New(),
		otherWS:     ids.New(),
		adminUser:   ids.New(),
		repUser:     ids.New(),
		otherUser:   ids.New(),
		adminRole:   ids.New(),
		managerRole: ids.New(),
		repRole:     ids.New(),
	}
	f.mux = membersTestMux(db)

	for _, ws := range []string{f.ws, f.otherWS} {
		mustExec(ctx, t, db,
			`INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
			ws, "w"+ws, "w"+ws)
	}

	mustExec(ctx, t, db,
		`INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		f.adminUser, f.ws, "admin-"+f.adminUser+"@example.com", "Admin")
	mustExec(ctx, t, db,
		`INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		f.repUser, f.ws, "rep-"+f.repUser+"@example.com", "Rep")
	mustExec(ctx, t, db,
		`INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`,
		f.otherUser, f.otherWS, "other-"+f.otherUser+"@example.com", "Other")

	// admin role carries workspace/manage_members; rep/manager do not.
	mustExec(ctx, t, db,
		`INSERT INTO role (id,workspace_id,key,is_system,permissions) VALUES ($1::uuid,$2::uuid,'admin',true,$3::jsonb)`,
		f.adminRole, f.ws, `{"workspace":{"manage_members":{"row_scope":"all"}}}`)
	mustExec(ctx, t, db,
		`INSERT INTO role (id,workspace_id,key,is_system,permissions) VALUES ($1::uuid,$2::uuid,'manager',false,$3::jsonb)`,
		f.managerRole, f.ws, `{"person":{"read":{"row_scope":"all"}}}`)
	mustExec(ctx, t, db,
		`INSERT INTO role (id,workspace_id,key,is_system,permissions) VALUES ($1::uuid,$2::uuid,'rep',false,$3::jsonb)`,
		f.repRole, f.ws, `{"person":{"read":{"row_scope":"own"}}}`)

	mustExec(ctx, t, db,
		`INSERT INTO role_assignment (workspace_id,role_id,user_id) VALUES ($1::uuid,$2::uuid,$3::uuid)`,
		f.ws, f.adminRole, f.adminUser)
	mustExec(ctx, t, db,
		`INSERT INTO role_assignment (workspace_id,role_id,user_id) VALUES ($1::uuid,$2::uuid,$3::uuid)`,
		f.ws, f.repRole, f.repUser)

	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, ws := range []string{f.ws, f.otherWS} {
			_, _ = db.ExecContext(cctx, `SELECT set_config('app.workspace_id',$1,false)`, ws)
			_, _ = db.ExecContext(cctx, `DELETE FROM audit_log WHERE workspace_id=$1::uuid`, ws)
			_, _ = db.ExecContext(cctx, `DELETE FROM role_assignment WHERE workspace_id=$1::uuid`, ws)
			_, _ = db.ExecContext(cctx, `DELETE FROM role WHERE workspace_id=$1::uuid`, ws)
			_, _ = db.ExecContext(cctx, `DELETE FROM app_user WHERE workspace_id=$1::uuid`, ws)
			_, _ = db.ExecContext(cctx, `DELETE FROM workspace WHERE id=$1::uuid`, ws)
		}
		db.Close()
	})
	return f
}

// req drives the test mux as a given principal (workspace + user via headers).
func (f *membersFixture) req(method, target, userID, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("X-Workspace-ID", f.ws)
	r.Header.Set("X-User-ID", userID)
	rr := httptest.NewRecorder()
	f.mux.ServeHTTP(rr, r)
	return rr
}

func (f *membersFixture) assignmentCount(ctx context.Context, t *testing.T, roleID, userID string) int {
	t.Helper()
	var n int
	if err := f.db.QueryRowContext(ctx,
		`SELECT count(*) FROM role_assignment WHERE workspace_id=$1::uuid AND role_id=$2::uuid AND user_id=$3::uuid`,
		f.ws, roleID, userID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestMembersAdminListsRoles(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	rr := f.req(http.MethodGet, "/roles", f.adminUser, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, key := range []string{"admin", "manager", "rep"} {
		if !strings.Contains(body, `"key":"`+key+`"`) {
			t.Errorf("roles response missing key %q: %s", key, body)
		}
	}
}

func TestMembersAdminListsMembers(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	rr := f.req(http.MethodGet, "/members", f.adminUser, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	// The admin's row carries its role key.
	if !strings.Contains(body, `"roles":["admin"]`) {
		t.Errorf("expected admin member with roles [admin]: %s", body)
	}
	if !strings.Contains(body, `"roles":["rep"]`) {
		t.Errorf("expected rep member with roles [rep]: %s", body)
	}
}

func TestMembersAdminAssignsRole(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	rr := f.req(http.MethodPost, "/members/"+f.repUser+"/roles", f.adminUser, `{"role_key":"manager"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if got := f.assignmentCount(ctx, t, f.managerRole, f.repUser); got != 1 {
		t.Fatalf("manager assignment count = %d, want 1", got)
	}
}

func TestMembersReassignIsIdempotent(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	for i := 0; i < 2; i++ {
		rr := f.req(http.MethodPost, "/members/"+f.repUser+"/roles", f.adminUser, `{"role_key":"manager"}`)
		if rr.Code != http.StatusCreated {
			t.Fatalf("attempt %d status = %d, want 201; body=%s", i, rr.Code, rr.Body.String())
		}
	}
	if got := f.assignmentCount(ctx, t, f.managerRole, f.repUser); got != 1 {
		t.Fatalf("after re-assign, count = %d, want exactly 1", got)
	}
}

func TestMembersNonAdminAssignForbidden(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	rr := f.req(http.MethodPost, "/members/"+f.repUser+"/roles", f.repUser, `{"role_key":"manager"}`)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
	if got := f.assignmentCount(ctx, t, f.managerRole, f.repUser); got != 0 {
		t.Fatalf("forbidden assign created %d rows, want 0", got)
	}
}

func TestMembersNonAdminListForbidden(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	rr := f.req(http.MethodGet, "/members", f.repUser, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
}

func TestMembersAdminRevokesNonLastRole(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	// Give the rep a manager role, then revoke it.
	if rr := f.req(http.MethodPost, "/members/"+f.repUser+"/roles", f.adminUser, `{"role_key":"manager"}`); rr.Code != http.StatusCreated {
		t.Fatalf("setup assign status = %d; body=%s", rr.Code, rr.Body.String())
	}
	rr := f.req(http.MethodDelete, "/members/"+f.repUser+"/roles/manager", f.adminUser, "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rr.Code, rr.Body.String())
	}
	if got := f.assignmentCount(ctx, t, f.managerRole, f.repUser); got != 0 {
		t.Fatalf("after revoke, count = %d, want 0", got)
	}
}

func TestMembersRevokeLastAdminConflict(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	rr := f.req(http.MethodDelete, "/members/"+f.adminUser+"/roles/admin", f.adminUser, "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rr.Code, rr.Body.String())
	}
	if got := f.assignmentCount(ctx, t, f.adminRole, f.adminUser); got != 1 {
		t.Fatalf("admin assignment count = %d, want 1 (still present)", got)
	}
}

func TestMembersCrossWorkspaceUserNotFound(t *testing.T) {
	ctx := context.Background()
	f := setupMembersFixture(ctx, t)
	// Assigning to a user in another workspace → 404 (never leak existence).
	rr := f.req(http.MethodPost, "/members/"+f.otherUser+"/roles", f.adminUser, `{"role_key":"manager"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("assign cross-ws status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
	// Revoking from a user in another workspace → 404 (the assignment isn't in ws).
	rr = f.req(http.MethodDelete, "/members/"+f.otherUser+"/roles/manager", f.adminUser, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("revoke cross-ws status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
	// The other-workspace user never appears in this workspace's member list.
	rr = f.req(http.MethodGet, "/members", f.adminUser, "")
	if strings.Contains(rr.Body.String(), f.otherUser) {
		t.Fatalf("cross-ws user leaked into /members: %s", rr.Body.String())
	}
}
