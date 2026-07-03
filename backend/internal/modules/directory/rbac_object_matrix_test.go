//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

type rbacCase struct {
	role       string
	object     string
	action     string
	method     string
	path       string
	wantStatus int
}

// loadUserPerms loads and merges the permissions JSONB for every role assigned to
// userID, returning the validated RolePermissions produced by the REAL crmauth
// validator. The DB query only *loads* raw JSONB; all decision logic lives in
// crmauth (ValidatePermissions / AuthorizePerms) — never re-implemented here.
func loadUserPerms(ctx context.Context, db *sql.DB, workspaceID, userID string) (crmauth.RolePermissions, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.permissions
		FROM role r JOIN role_assignment ra ON ra.role_id=r.id
		WHERE ra.workspace_id=$1::uuid AND ra.user_id=$2::uuid`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	merged := crmauth.RolePermissions{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var rawPerms map[string]any
		if err := json.Unmarshal(raw, &rawPerms); err != nil {
			return nil, err
		}
		// Validate + parse through the real crmauth function.
		perms, err := crmauth.ValidatePermissions(rawPerms)
		if err != nil {
			return nil, err
		}
		// Union the validated entries across all of the user's roles.
		for obj, entry := range perms {
			existing, ok := merged[obj]
			if !ok {
				merged[obj] = entry
				continue
			}
			for action, rule := range entry.Actions {
				existing.Actions[action] = rule
			}
			merged[obj] = existing
		}
	}
	return merged, rows.Err()
}

// userCanDo decides allow/deny by loading the role permissions from the DB and
// delegating the actual decision to crmauth.AuthorizePerms — the exported symbol
// under test. No permission logic is duplicated in the test.
func userCanDo(ctx context.Context, db *sql.DB, workspaceID, userID, object, action string) bool {
	perms, err := loadUserPerms(ctx, db, workspaceID, userID)
	if err != nil {
		return false
	}
	return crmauth.AuthorizePerms(perms, object, action) == nil
}

// buildRBACMux returns an http.Handler that enforces role-based access by looking
// up the test user's role permissions and checking them (via crmauth) before
// delegating to core handlers.
func buildRBACMux(t *testing.T, db *sql.DB) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	personStore := crmcore.NewPersonStore(db)
	authWrap := func(object, action string, h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := crmctx.From(r.Context())
			if !ok {
				http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized) //nolint:forbidigo // test mock handler, not a production JSON path
				return
			}
			if !userCanDo(r.Context(), db, p.TenantID, p.UserID, object, action) {
				http.Error(w, `{"code":"forbidden"}`, http.StatusForbidden) //nolint:forbidigo // test mock handler, not a production JSON path
				return
			}
			h.ServeHTTP(w, r)
		})
	}
	mux.Handle("/people", authWrap("person", "read", peopletransport.NewPersonHandler(personStore)))
	return mux
}

// seedRBACFixtures creates (idempotently) the workspace, roles with permission
// JSONB, users, and role_assignments the object matrix exercises. margince is a
// superuser/bypassrls role, so these direct inserts bypass RLS. The permissions
// JSONB written here is exactly what crmauth.ValidatePermissions parses at read
// time, so the test exercises the real load->validate->authorize path.
func seedRBACFixtures(ctx context.Context, t *testing.T, db *sql.DB, wsID string, roleUserIDs map[string]string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'rbac-obj','rbac-obj','EUR') ON CONFLICT DO NOTHING`,
		wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	rolePerms := map[string]string{
		"admin":     `{"person":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"pipeline":{"read":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}}}`,
		"rep":       `{"person":{"read":{"row_scope":"own"},"create":{"row_scope":"own"}},"deal":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"archive":{"row_scope":"own"}}}`,
		"read_only": `{"person":{"read":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}}}`,
		"ops":       `{"pipeline":{"read":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}}}`,
	}
	// Deterministic role IDs derived from the role user IDs' last byte.
	roleIDs := map[string]string{
		"admin":     "00000000-0000-0000-0020-000000000001",
		"rep":       "00000000-0000-0000-0020-000000000002",
		"read_only": "00000000-0000-0000-0020-000000000003",
		"ops":       "00000000-0000-0000-0020-000000000004",
	}

	for role, userID := range roleUserIDs {
		roleID := roleIDs[role]
		if _, err := db.ExecContext(ctx,
			`INSERT INTO app_user(id,workspace_id,email,display_name)
			 VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			userID, wsID, role+"@rbac-obj.example.com", role); err != nil {
			t.Fatalf("seed user %s: %v", role, err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO role(id,workspace_id,key,is_system,permissions)
			 VALUES($1,$2,$3,true,$4::jsonb) ON CONFLICT DO NOTHING`,
			roleID, wsID, role, rolePerms[role]); err != nil {
			t.Fatalf("seed role %s: %v", role, err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO role_assignment(workspace_id,role_id,user_id)
			 VALUES($1,$2,$3)
			 ON CONFLICT (role_id,user_id,COALESCE(team_id,'00000000-0000-0000-0000-000000000000'::uuid)) DO NOTHING`,
			wsID, roleID, userID); err != nil {
			t.Fatalf("seed role_assignment %s: %v", role, err)
		}
	}
}

func TestRBACObjectMatrix(t *testing.T) {
	db := mustDB(t)
	ctx := context.Background()

	const wsID = "00000000-0000-0000-0000-000000000001"

	cases := []rbacCase{
		// admin — can read person
		{role: "admin", object: "person", action: "read", method: "GET", path: "/people", wantStatus: 200},
		// rep — can create person (POST is correct for create)
		{role: "rep", object: "person", action: "create", method: "POST", path: "/people", wantStatus: 200},
		// read_only — cannot create person (no "create" in permissions)
		{role: "read_only", object: "person", action: "create", method: "POST", path: "/people", wantStatus: 403},
		// read_only — can read person
		{role: "read_only", object: "person", action: "read", method: "GET", path: "/people", wantStatus: 200},
		// admin/person/archive → allowed
		{role: "admin", object: "person", action: "archive", method: "DELETE", path: "/people", wantStatus: 200},
		// rep/deal/archive → allowed
		{role: "rep", object: "deal", action: "archive", method: "DELETE", path: "/people", wantStatus: 200},
		// read_only/deal/archive → denied (403)
		{role: "read_only", object: "deal", action: "archive", method: "DELETE", path: "/people", wantStatus: 403},
		// ops/pipeline/create → denied (403)
		{role: "ops", object: "pipeline", action: "create", method: "POST", path: "/people", wantStatus: 403},
		// ops/report/read → allowed
		{role: "ops", object: "report", action: "read", method: "GET", path: "/people", wantStatus: 200},
	}

	// User IDs: admin=...001, rep=...002, read_only=...003, ops=...004
	roleUserIDs := map[string]string{
		"admin":     "00000000-0000-0000-0010-000000000001",
		"rep":       "00000000-0000-0000-0010-000000000002",
		"read_only": "00000000-0000-0000-0010-000000000003",
		"ops":       "00000000-0000-0000-0010-000000000004",
	}

	// Self-seed roles/users/assignments so the test is hermetic and exercises the
	// real crmauth load->validate->authorize path end-to-end.
	seedRBACFixtures(ctx, t, db, wsID, roleUserIDs)

	mux := buildRBACMux(t, db)

	for _, tc := range cases {
		t.Run(tc.role+"/"+tc.object+"/"+tc.action, func(t *testing.T) {
			userID, ok := roleUserIDs[tc.role]
			if !ok {
				t.Fatalf("no test user for role %q", tc.role)
			}
			// RBAC decision goes through the real crmauth functions via userCanDo.
			canDo := userCanDo(ctx, db, wsID, userID, tc.object, tc.action)
			if tc.wantStatus == 200 && !canDo {
				t.Errorf("role %q should be allowed %q on %q", tc.role, tc.action, tc.object)
			}
			if tc.wantStatus == 403 && canDo {
				t.Errorf("role %q must NOT be allowed %q on %q", tc.role, tc.action, tc.object)
			}

			// HTTP-level check via mux for the read path (the mux guards person/read).
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rctx := crmctx.With(req.Context(), crmctx.Principal{UserID: userID, TenantID: wsID})
			req = req.WithContext(rctx)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			// The mux enforces person/read; assert it matches the read decision.
			readAllowed := userCanDo(ctx, db, wsID, userID, "person", "read")
			if readAllowed && w.Code == http.StatusForbidden {
				t.Errorf("mux returned 403 but read is allowed for role %q (body: %s)", tc.role, w.Body.String())
			}
			if !readAllowed && w.Code != http.StatusForbidden {
				t.Errorf("mux must return 403 for role %q without person/read, got %d", tc.role, w.Code)
			}
		})
	}
}
