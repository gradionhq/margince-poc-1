package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// E11 role-assignment admin surface. These four handlers list a workspace's
// roles/members and assign/revoke role assignments over the existing
// role/role_assignment tables (mig 000005). They are gated by the
// workspace/manage_members RBAC action (not a CRUDA method-derived action), so
// they bypass httpserver.RbacMiddleware and use RequireManageMembers instead. Every mutation
// is workspace-scoped, idempotent on assign, last-admin-protected on revoke, and
// writes exactly one audit_log row atomic with the mutation.

// roleJSON mirrors the contract Role schema (role table has no name column, so
// name = key — see plan + mig 000005).
type roleJSON struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	IsSystem    bool           `json:"is_system"`
	Permissions map[string]any `json:"permissions"`
}

// memberJSON mirrors the contract Member schema.
type memberJSON struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Status      string   `json:"status"`
	IsAgent     bool     `json:"is_agent"`
	Roles       []string `json:"roles"`
}

// assignRoleBody is the POST /members/{user_id}/roles request body. One of
// role_key / role_id is required.
type assignRoleBody struct {
	RoleKey string `json:"role_key"`
	RoleID  string `json:"role_id"`
}

// RequireManageMembers gates next behind the workspace/manage_members permission.
// It mirrors httpserver.RbacMiddleware (load the principal's merged role permissions, then
// AuthorizePerms) but for the non-method-derived action. 401 with no principal,
// 403 without the permission. Composes in cmd/api/routes.go as
// workspaceWrap(RequireManageMembers(db, handler)).
func RequireManageMembers(db *sql.DB, next http.Handler) http.Handler {
	return httpserver.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := crmctx.From(r.Context()) // guaranteed by httpserver.RequireAuth above
		perms, err := httpserver.LoadRolePermissions(r.Context(), db, p.TenantID, p.UserID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		if err := crmauth.AuthorizePerms(perms, "workspace", "manage_members"); err != nil {
			httpserver.WriteProblem(w, http.StatusForbidden, httpserver.CodeForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// HandleListRoles GET /roles — the workspace's assignable roles, ordered by key.
func HandleListRoles(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := crmctx.From(r.Context())
		rows, err := db.QueryContext(r.Context(), `
			SELECT id, key, is_system, permissions
			FROM role WHERE workspace_id=$1::uuid ORDER BY key`, p.TenantID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		defer func() { _ = rows.Close() }()
		data := []roleJSON{}
		for rows.Next() {
			var id, key string
			var isSystem bool
			var rawPerms []byte
			if err := rows.Scan(&id, &key, &isSystem, &rawPerms); err != nil {
				httpserver.WriteInternal(w)
				return
			}
			perms := map[string]any{}
			_ = json.Unmarshal(rawPerms, &perms)
			data = append(data, roleJSON{
				ID: id, WorkspaceID: p.TenantID, Key: key, Name: key,
				IsSystem: isSystem, Permissions: perms,
			})
		}
		if err := rows.Err(); err != nil {
			httpserver.WriteInternal(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	}
}

// HandleListMembers GET /members — workspace members with their role keys.
func HandleListMembers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := crmctx.From(r.Context())
		rows, err := db.QueryContext(r.Context(), `
			SELECT id, email, display_name, status, COALESCE(is_agent,false)
			FROM app_user WHERE workspace_id=$1::uuid AND archived_at IS NULL
			ORDER BY email`, p.TenantID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		defer func() { _ = rows.Close() }()
		data := []memberJSON{}
		for rows.Next() {
			var m memberJSON
			if err := rows.Scan(&m.UserID, &m.Email, &m.DisplayName, &m.Status, &m.IsAgent); err != nil {
				httpserver.WriteInternal(w)
				return
			}
			data = append(data, m)
		}
		if err := rows.Err(); err != nil {
			httpserver.WriteInternal(w)
			return
		}
		// Attach role keys per member (separate pass — rows must be closed first).
		for i := range data {
			roles, err := loadUserRoleKeys(r.Context(), db, p.TenantID, data[i].UserID)
			if err != nil {
				httpserver.WriteInternal(w)
				return
			}
			data[i].Roles = roles
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	}
}

// resolveWorkspaceRole looks up a role id within the workspace by key or id
// (whichever the body carries), returning sql.ErrNoRows when it does not exist.
func resolveWorkspaceRole(ctx context.Context, db *sql.DB, ws string, body assignRoleBody) (string, error) {
	var roleID string
	var err error
	if body.RoleKey != "" {
		err = db.QueryRowContext(ctx,
			`SELECT id FROM role WHERE workspace_id=$1::uuid AND key=$2`, ws, body.RoleKey).Scan(&roleID)
	} else {
		err = db.QueryRowContext(ctx,
			`SELECT id FROM role WHERE workspace_id=$1::uuid AND id=$2::uuid`, ws, body.RoleID).Scan(&roleID)
	}
	return roleID, err
}

// assignRoleTx idempotently inserts the (role, user) assignment and writes exactly
// one audit_log row in the SAME tx (mutation + audit atomic). A re-assign returns
// the existing row id so the audit entry is still written.
func assignRoleTx(ctx context.Context, db *sql.DB, ws, roleID, userID string) error {
	return database.WithWorkspaceTx(ctx, db, ws, func(tx *sql.Tx) error {
		var assignmentID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO role_assignment (workspace_id, role_id, user_id)
			VALUES ($1::uuid,$2::uuid,$3::uuid)
			ON CONFLICT (role_id, user_id, COALESCE(team_id,'00000000-0000-0000-0000-000000000000'::uuid))
			DO NOTHING
			RETURNING id`, ws, roleID, userID).Scan(&assignmentID)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `
				SELECT id FROM role_assignment
				WHERE workspace_id=$1::uuid AND role_id=$2::uuid AND user_id=$3::uuid AND team_id IS NULL`,
				ws, roleID, userID).Scan(&assignmentID)
		}
		if err != nil {
			return err
		}

		e := crmaudit.EntryFromPrincipal(ctx, "assign", "role_assignment", &assignmentID, nil, nil)
		_, err = crmaudit.WriteTx(ctx, tx, e)
		return err
	})
}

// HandleAssignRole POST /members/{user_id}/roles — idempotent assign.
func HandleAssignRole(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := crmctx.From(r.Context())
		ws := p.TenantID
		userID := r.PathValue("user_id")

		var body assignRoleBody
		if err := readJSON(r, &body); err != nil {
			httpserver.WriteProblem(w, http.StatusBadRequest, httpserver.CodeBadRequest)
			return
		}
		if body.RoleKey == "" && body.RoleID == "" {
			httpserver.WriteProblem(w, http.StatusUnprocessableEntity, httpserver.CodeValidation)
			return
		}

		// Resolve the role within the workspace, then verify the target user is in it.
		// A missing role OR a non-member user → identical 404 (never leak existence).
		roleID, err := resolveWorkspaceRole(r.Context(), db, ws, body)
		if errors.Is(err, sql.ErrNoRows) {
			httpserver.WriteProblem(w, http.StatusNotFound, httpserver.CodeNotFound)
			return
		}
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		var exists int
		err = db.QueryRowContext(r.Context(),
			`SELECT 1 FROM app_user WHERE id=$1::uuid AND workspace_id=$2::uuid`, userID, ws).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			httpserver.WriteProblem(w, http.StatusNotFound, httpserver.CodeNotFound)
			return
		}
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}

		if err := assignRoleTx(r.Context(), db, ws, roleID, userID); err != nil {
			httpserver.WriteInternal(w)
			return
		}

		m, err := loadMember(r.Context(), db, ws, userID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		writeJSON(w, http.StatusCreated, m)
	}
}

// HandleRevokeRole DELETE /members/{user_id}/roles/{role_key} — last-admin-guarded.
func HandleRevokeRole(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := crmctx.From(r.Context())
		ws := p.TenantID
		userID := r.PathValue("user_id")
		roleKey := r.PathValue("role_key")

		// Resolve the role within the workspace (404 if unknown).
		var roleID string
		err := db.QueryRowContext(r.Context(),
			`SELECT id FROM role WHERE workspace_id=$1::uuid AND key=$2`, ws, roleKey).Scan(&roleID)
		if errors.Is(err, sql.ErrNoRows) {
			httpserver.WriteProblem(w, http.StatusNotFound, httpserver.CodeNotFound)
			return
		}
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}

		// Last-admin guard: never strand a workspace without an admin.
		if roleKey == "admin" {
			var adminCount int
			if err := db.QueryRowContext(r.Context(), `
				SELECT count(*) FROM role_assignment ra
				JOIN role r ON r.id = ra.role_id
				WHERE ra.workspace_id=$1::uuid AND r.key='admin'`, ws).Scan(&adminCount); err != nil {
				httpserver.WriteInternal(w)
				return
			}
			var holdsAdmin int
			holdsErr := db.QueryRowContext(r.Context(),
				`SELECT 1 FROM role_assignment WHERE workspace_id=$1::uuid AND role_id=$2::uuid AND user_id=$3::uuid`,
				ws, roleID, userID).Scan(&holdsAdmin)
			userHoldsAdmin := holdsErr == nil
			if !userHoldsAdmin && !errors.Is(holdsErr, sql.ErrNoRows) {
				httpserver.WriteInternal(w)
				return
			}
			if userHoldsAdmin && adminCount <= 1 {
				httpserver.WriteProblem(w, http.StatusConflict, httpserver.CodeConflict)
				return
			}
		}

		err = database.WithWorkspaceTx(r.Context(), db, ws, func(tx *sql.Tx) error {
			var assignmentID string
			e := tx.QueryRowContext(r.Context(), `
				DELETE FROM role_assignment
				WHERE workspace_id=$1::uuid AND role_id=$2::uuid AND user_id=$3::uuid
				RETURNING id`, ws, roleID, userID).Scan(&assignmentID)
			if e != nil {
				return e
			}
			entry := crmaudit.EntryFromPrincipal(r.Context(), "archive", "role_assignment", &assignmentID, nil, nil)
			_, e = crmaudit.WriteTx(r.Context(), tx, entry)
			return e
		})
		if errors.Is(err, sql.ErrNoRows) {
			httpserver.WriteProblem(w, http.StatusNotFound, httpserver.CodeNotFound)
			return
		}
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// loadMember builds the current Member view for one user (identity + role keys).
func loadMember(ctx context.Context, db *sql.DB, workspaceID, userID string) (memberJSON, error) {
	m := memberJSON{UserID: userID}
	err := db.QueryRowContext(ctx, `
		SELECT email, display_name, status, COALESCE(is_agent,false)
		FROM app_user WHERE id=$1::uuid AND workspace_id=$2::uuid`, userID, workspaceID).
		Scan(&m.Email, &m.DisplayName, &m.Status, &m.IsAgent)
	if err != nil {
		return m, err
	}
	roles, err := loadUserRoleKeys(ctx, db, workspaceID, userID)
	if err != nil {
		return m, err
	}
	m.Roles = roles
	return m, nil
}
