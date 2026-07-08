// Package transport holds the identity module's HTTP handlers (auth,
// passports, member/role admin), extracted from the cmd/api composition root
// (1c restructure, task-3-brief.md). package main → package transport is the
// one authorized rename for this extraction (mirrors the httpserver
// extraction); mounting stays in cmd/api/routes.go (decision A2).
package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	_ "github.com/lib/pq" // postgres driver registration, carried over unchanged from this file's original cmd/api/auth_handler.go (poc)

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	platformauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// Field names reused across the auth/member responses and the request log /
// metric labels, hoisted so the repeated string literals stay spelled one way.
const (
	keyName        = "name"
	keyStatus      = "status"
	keyWorkspaceID = "workspace_id"
	keyCreatedAt   = "created_at"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// clientIP extracts the caller's IP for session provenance (session.ip, D4).
// RemoteAddr only — this deployment has no trusted reverse-proxy chain
// configured, so honoring X-Forwarded-For would let a client spoof any IP.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// defaultConsentPurposes are seeded into every newly-signed-up workspace (D2
// step 7 — consent_purpose became per-workspace in migration 000070; nothing
// previously seeded a purpose set for new workspaces). This literal is
// independent of that migration's own backfill for pre-existing workspaces,
// which seeds directly from the old global rows; the two lists happen to share
// the same 4 entries today but are not the same code path.
var defaultConsentPurposes = []struct{ Key, Label string }{
	{"marketing_email", "Email marketing communications"},
	{"marketing_phone", "Phone marketing communications"},
	{"profiling", "Profiling and personalisation"},
	{"product_updates", "Product update notifications"},
}

// adminPermissionsJSON is the permission JSONB granted to a workspace's bootstrap
// admin. It mirrors the seeded `admin` role (see backend/seed/dev.sql): full CRUA on the
// core objects, read on report, and mint/read/revoke on passport.
var adminPermissionsJSON = func() string {
	all := func(actions ...string) map[string]any {
		m := make(map[string]any, len(actions))
		for _, a := range actions {
			m[a] = map[string]any{"row_scope": "all"}
		}
		return m
	}
	crua := []string{platformauth.ActionRead, platformauth.ActionCreate, platformauth.ActionUpdate, platformauth.ActionArchive}
	perms := map[string]any{
		platformauth.ObjPerson:        all(crua...),
		"organization":                all(crua...),
		"deal":                        all(crua...),
		"pipeline":                    all(crua...),
		"activity":                    all(crua...),
		"lead":                        all(crua...),
		platformauth.ObjProduct:       all(crua...),
		platformauth.ObjOfferTemplate: all(crua...),
		"report":                      all(platformauth.ActionRead),
		"passport":                    all(platformauth.ActionRead, platformauth.ActionCreate, platformauth.ActionArchive),
		"approval":                    all(platformauth.ActionRead, "decide"),
		"workspace":                   map[string]any{"manage_members": map[string]any{"row_scope": "all"}},
		"record_grant":                all(platformauth.ActionRead, platformauth.ActionCreate, platformauth.ActionArchive),
		"custom_field":                all(platformauth.ActionRead, platformauth.ActionCreate),
	}
	b, err := json.Marshal(perms)
	if err != nil {
		panic("marshal admin permissions: " + err.Error())
	}
	return string(b)
}()

// HandleCreateWorkspace POST /workspaces — bootstrap workspace + admin in one tx.
func HandleCreateWorkspace(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name             string `json:"name"`
			Slug             string `json:"slug"`
			BaseCurrency     string `json:"base_currency"`
			AdminEmail       string `json:"admin_email"`
			AdminPassword    string `json:"admin_password"`
			AdminDisplayName string `json:"admin_display_name"`
		}
		if err := readJSON(r, &req); err != nil {
			httpserver.WriteProblem(w, http.StatusBadRequest, httpserver.CodeBadRequest)
			return
		}
		hash, err := crmauth.HashPassword(req.AdminPassword)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		wsID := ids.New()
		userID := ids.New()
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		defer tx.Rollback() //nolint:errcheck // rolled back unless Commit succeeds; rollback error is moot
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,$4)`,
			wsID, req.Name, req.Slug, req.BaseCurrency); err != nil {
			httpserver.WriteProblem(w, http.StatusConflict, httpserver.CodeConflict)
			return
		}
		// D2 step 7: seed this workspace's own consent_purpose rows in the same
		// tx — without this, every person_consent/consent_event insert (which
		// FKs to consent_purpose) would fail for a workspace created after
		// migration 000070 widened consent_purpose to per-workspace.
		for _, cp := range defaultConsentPurposes {
			if _, err := tx.ExecContext(r.Context(),
				`INSERT INTO consent_purpose (workspace_id, key, label) VALUES ($1::uuid, $2, $3)`,
				wsID, cp.Key, cp.Label); err != nil {
				httpserver.WriteInternal(w)
				return
			}
		}
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO app_user (id, workspace_id, email, display_name, password_hash)
			 VALUES ($1::uuid,$2::uuid,$3,$4,$5)`,
			userID, wsID, strings.ToLower(req.AdminEmail), req.AdminDisplayName, hash); err != nil {
			httpserver.WriteProblem(w, http.StatusConflict, httpserver.CodeConflict)
			return
		}
		// Grant the bootstrap user a full admin role, otherwise it logs in but is
		// denied on every object route (RBAC loads zero permissions). The role +
		// assignment ride the same transaction so a workspace is never created
		// without an admin who can actually use it.
		var roleID string
		if err := tx.QueryRowContext(r.Context(),
			`INSERT INTO role (workspace_id, key, is_system, permissions)
			 VALUES ($1::uuid, 'admin', true, $2::jsonb) RETURNING id`,
			wsID, adminPermissionsJSON).Scan(&roleID); err != nil {
			httpserver.WriteInternal(w)
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO role_assignment (workspace_id, role_id, user_id)
			 VALUES ($1::uuid, $2::uuid, $3::uuid)`,
			wsID, roleID, userID); err != nil {
			httpserver.WriteInternal(w)
			return
		}
		if err := tx.Commit(); err != nil {
			httpserver.WriteInternal(w)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": wsID, keyName: req.Name, "slug": req.Slug,
			"base_currency": req.BaseCurrency, keyCreatedAt: time.Now().UTC(),
		})
	}
}

// HandleLogin POST /auth/login
func HandleLogin(db *sql.DB, sessions *crmauth.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := readJSON(r, &req); err != nil {
			httpserver.WriteProblem(w, http.StatusBadRequest, httpserver.CodeBadRequest)
			return
		}
		var userID, workspaceID, hash string
		err := db.QueryRowContext(r.Context(),
			`SELECT id, workspace_id, password_hash FROM app_user
			 WHERE lower(email)=$1 AND status='active' AND archived_at IS NULL`,
			strings.ToLower(req.Email)).Scan(&userID, &workspaceID, &hash)
		if errors.Is(err, sql.ErrNoRows) || !crmauth.VerifyPassword(hash, req.Password) {
			httpserver.WriteProblem(w, http.StatusUnauthorized, httpserver.CodeUnauthorized)
			return
		}
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		rawToken, err := sessions.Create(r.Context(), workspaceID, userID, r.UserAgent(), clientIP(r))
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		// Secure mirrors the connection (true under TLS) so the dev HTTP login path
		// still works; the cookie is always HttpOnly + SameSite=Strict.
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure is connection-conditional by design (see comment); HttpOnly+SameSite always set
			Name:     crmauth.CookieName,
			Value:    rawToken,
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
			MaxAge:   int((24 * time.Hour).Seconds()),
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"user_id": userID, keyWorkspaceID: workspaceID,
		})
	}
}

// HandleLogout POST /auth/logout
func HandleLogout(sessions *crmauth.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(crmauth.CookieName)
		if err == nil {
			if rec, err := sessions.Lookup(r.Context(), cookie.Value); err == nil {
				// Soft-revoke on logout (D4): the restored revoked_at column is
				// the real writer for "this session must stop authenticating",
				// not a hard delete.
				_ = sessions.Revoke(r.Context(), rec.ID, rec.WorkspaceID)
			}
		}
		// Mirror the login cookie's attributes on the clearing cookie so browsers
		// match and evict it. Secure is connection-conditional (dev HTTP support).
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure is connection-conditional by design (see comment); HttpOnly+SameSite always set
			Name:     crmauth.CookieName,
			Value:    "",
			MaxAge:   -1,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
		})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// HandleCreatePassport POST /passports
func HandleCreatePassport(db *sql.DB, passports *crmauth.PassportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := crmctx.From(r.Context())
		if !ok {
			httpserver.WriteProblem(w, http.StatusUnauthorized, httpserver.CodeUnauthorized)
			return
		}
		var req struct {
			Scopes           []string `json:"scopes"`
			ExpiresInSeconds int      `json:"expires_in_seconds"`
		}
		if err := readJSON(r, &req); err != nil {
			httpserver.WriteProblem(w, http.StatusBadRequest, httpserver.CodeBadRequest)
			return
		}
		// Load human's scopes from their roles (simplified: query role_assignment + role).
		humanScopes, err := loadHumanScopes(r.Context(), db, p.TenantID, p.UserID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		if err := crmauth.CheckScopeSubset(req.Scopes, humanScopes); err != nil {
			httpserver.WriteProblem(w, http.StatusUnprocessableEntity, httpserver.CodeScopeExceeded)
			return
		}
		// No distinct on-behalf-of principal is carried by this request today,
		// so on_behalf_of defaults to the granting human themselves.
		rawToken, rec, err := passports.Create(r.Context(), p.TenantID, p.UserID, p.UserID, "", req.Scopes,
			time.Duration(req.ExpiresInSeconds)*time.Second)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		// Fetch expires_at + revoked_at from the passport row (contract-required fields).
		var expiresAt time.Time
		var revokedAt *time.Time
		_ = db.QueryRowContext(r.Context(),
			`SELECT expires_at, revoked_at FROM passport WHERE id=$1::uuid`, rec.ID).
			Scan(&expiresAt, &revokedAt)
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": rec.ID, "granted_by": rec.GrantedBy, "scopes": rec.Scopes,
			"token": rawToken, "expires_at": expiresAt, "revoked_at": revokedAt,
		})
	}
}

// HandleRevokePassport DELETE /passports/{id}
func HandleRevokePassport(passports *crmauth.PassportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := crmctx.From(r.Context())
		if !ok {
			httpserver.WriteProblem(w, http.StatusUnauthorized, httpserver.CodeUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/passports/")
		if err := passports.Revoke(r.Context(), id, p.TenantID); err != nil {
			if errors.Is(err, crmauth.ErrNotFound) {
				httpserver.WriteProblem(w, http.StatusNotFound, httpserver.CodeNotFound)
				return
			}
			httpserver.WriteInternal(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// loadUserRoleKeys returns the distinct role keys assigned to the user, ordered
// for stable output. Always returns a non-nil slice so the JSON encodes as [].
func loadUserRoleKeys(ctx context.Context, db *sql.DB, workspaceID, userID string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT r.key
		FROM role r
		JOIN role_assignment ra ON ra.role_id = r.id
		WHERE ra.workspace_id=$1::uuid AND ra.user_id=$2::uuid
		ORDER BY r.key`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// loadUserTeamIDs returns the team UUIDs the user belongs to. Always returns a
// non-nil slice so the JSON encodes as [].
func loadUserTeamIDs(ctx context.Context, db *sql.DB, workspaceID, userID string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT team_id
		FROM team_membership
		WHERE workspace_id=$1::uuid AND user_id=$2::uuid
		ORDER BY team_id`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// loadHumanScopes derives scope strings ("action:object") from the user's roles'
// permissions JSONB. It joins role_assignment to role, validates each permissions
// blob, and dedupes the resulting scopes.
func loadHumanScopes(ctx context.Context, db *sql.DB, workspaceID, userID string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.permissions
		FROM role r
		JOIN role_assignment ra ON ra.role_id = r.id
		WHERE ra.workspace_id=$1::uuid AND ra.user_id=$2::uuid`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var allScopes []string
	scopeSet := map[string]bool{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var permsMap map[string]any
		if err := json.Unmarshal(raw, &permsMap); err != nil {
			continue
		}
		perms, err := crmauth.ValidatePermissions(permsMap)
		if err != nil {
			continue
		}
		for _, s := range crmauth.LoadUserScopesFromPerms(perms) {
			if !scopeSet[s] {
				scopeSet[s] = true
				allScopes = append(allScopes, s)
			}
		}
	}
	return allScopes, rows.Err()
}

// HandleMe GET /me
func HandleMe(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := crmctx.From(r.Context())
		if !ok {
			httpserver.WriteProblem(w, http.StatusUnauthorized, httpserver.CodeUnauthorized)
			return
		}
		var email, displayName, status string
		_ = db.QueryRowContext(r.Context(),
			`SELECT email, display_name, status FROM app_user WHERE id=$1::uuid`, p.UserID).
			Scan(&email, &displayName, &status)
		if status == "" {
			status = "active"
		}

		// MeResponse requires roles + teams (contract: MeResponse).
		roles, err := loadUserRoleKeys(r.Context(), db, p.TenantID, p.UserID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		teams, err := loadUserTeamIDs(r.Context(), db, p.TenantID, p.UserID)
		if err != nil {
			httpserver.WriteInternal(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"id": p.UserID, keyWorkspaceID: p.TenantID,
				"email": email, "display_name": displayName,
				keyStatus: status, "is_agent": p.IsAgent,
			},
			"roles": roles,
			"teams": teams,
		})
	}
}
