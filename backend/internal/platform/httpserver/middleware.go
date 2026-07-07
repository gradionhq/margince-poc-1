// Package httpserver holds the shared HTTP error writer and the middleware
// stack (session/workspace/RBAC) previously colocated with the cmd/api
// composition root. Extracted verbatim (1c restructure, task-3-brief.md) —
// package main → package httpserver is the one authorized rename for this
// extraction; behavior is unchanged.
package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	obs "github.com/gradionhq/margince/backend/internal/shared/kernel/obs"
	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

// keyStatus is the structured-log field name for the response status, local to
// this package's own logging (metrics.go in cmd/api has its own copy for its
// Prometheus label name — same literal, unrelated concern, not worth coupling
// two packages over a 6-character string).
const keyStatus = "status"

// statusRecorder is a minimal http.ResponseWriter wrapper that captures the
// written status code. cmd/api's metrics.go declares its own copy for the same
// reason as keyStatus above: a 3-line type, not worth an import-boundary
// coupling in either direction (package main cannot be imported, and exporting
// this out of httpserver solely for metrics.go's benefit would be a
// content/visibility change beyond this extraction's scope).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) { r.status = code; r.ResponseWriter.WriteHeader(code) }

// LogRequest logs one line per HTTP request: method, path, status, latency, trace_id.
func LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		slog.Info(
			"request",
			"method", r.Method,
			"path", r.URL.Path,
			keyStatus, sr.status,
			"ms", time.Since(start).Milliseconds(),
			"trace_id", obs.TraceID(r.Context()),
		)
	})
}

// SessionMiddleware reads crm_session cookie → LookupSession → crmctx injection.
// Also propagates W3C traceparent header into the ctx trace (mints a fresh trace if absent).
// Falls through silently if no cookie (RequireAuth enforces presence).
func SessionMiddleware(v session.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Derive every context below from this single inbound request context so
			// the chain stays inheritance-clean (contextcheck): build ctx up, then
			// install it on r exactly once before delegating.
			ctx := r.Context()

			// Propagate or mint a W3C traceparent.
			if tid, sid, ok := obs.ParseTraceparent(r.Header.Get("traceparent")); ok {
				ctx = obs.WithTrace(ctx, tid, sid)
			} else {
				ctx = obs.WithTrace(ctx, obs.NewTraceID(), obs.NewSpanID())
			}

			if cookie, err := r.Cookie(session.CookieName); err == nil && cookie.Value != "" {
				if p, ok := v.LookupSession(ctx, cookie.Value); ok {
					ctx = crmctx.With(ctx, p)
				}
			}
			// Also check Authorization: Bearer <passport_token> for agent calls.
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				rawToken := strings.TrimPrefix(auth, "Bearer ")
				if p, ok := v.LookupPassport(ctx, rawToken); ok {
					ctx = crmctx.With(ctx, p)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth returns 401 if no Principal is in context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := crmctx.From(r.Context()); !ok {
			WriteProblem(w, http.StatusUnauthorized, CodeUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Canonical RBAC action names, shared by MethodToAction and the explicit gates.
// ActionCreate lives here too (moved from cmd/api/auth_handler.go, its
// original poc file) so the four-member action-name group stays cohesive
// instead of splitting arbitrarily across the package boundary the httpserver
// extraction introduces; cmd/api now references it qualified.
const (
	ActionRead    = "read"
	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionArchive = "archive"
)

// ObjPerson is the canonical RBAC object name for people, shared by the
// admin-perms seed and the /people route wiring.
const ObjPerson = "person"

// ObjDeal is the canonical RBAC object name for deals, shared by the
// admin-perms seed and the /deals route wiring.
const ObjDeal = "deal"

// ObjOrganization is the canonical RBAC object name for organizations, shared
// by the /organizations route registration and its RBAC checks.
const ObjOrganization = "organization"

// ObjPipeline is the canonical RBAC object name for pipelines, shared by the
// admin-perms seed and the /pipelines route wiring.
const ObjPipeline = "pipeline"

// ObjStage is the canonical RBAC object name for stages, shared by the
// admin-perms seed and the /stages route wiring.
const ObjStage = "stage"

// ObjPartner is the canonical RBAC object name for the partner extension,
// shared by the /organizations/{id}/partner and /partners routes.
const ObjPartner = "partner"

// ObjRelationship is the canonical RBAC object name for the generic
// employment/deal_stakeholder edge, shared by the /relationships route
// wiring. GET /deals/{id}/stakeholders uses ObjDeal instead because it is a
// deal-scoped read.
const ObjRelationship = "relationship"

// ObjActivity is the canonical RBAC object name for activities, shared by
// the /activities route wiring and the seeded role_permission rows.
const ObjActivity = "activity"

// ObjRecordGrant is the canonical RBAC object name for record grants, shared
// by the /record-grants route wiring and approval-gating.
const ObjRecordGrant = "record_grant"

// MethodToAction maps an HTTP method to the canonical RBAC action name.
func MethodToAction(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet:
		return ActionRead
	case http.MethodPost:
		return ActionCreate
	case http.MethodPatch, http.MethodPut:
		return ActionUpdate
	case http.MethodDelete:
		return ActionArchive
	default:
		return ActionRead
	}
}

// LoadRolePermissions queries role r JOIN role_assignment ra for the principal's
// workspace+user and returns the merged, validated RolePermissions.
func LoadRolePermissions(ctx context.Context, db *sql.DB, workspaceID, userID string) (session.RolePermissions, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.permissions
		FROM role r
		JOIN role_assignment ra ON ra.role_id = r.id
		WHERE ra.workspace_id=$1::uuid AND ra.user_id=$2::uuid`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	merged := session.RolePermissions{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var rawPerms map[string]any
		if err := json.Unmarshal(raw, &rawPerms); err != nil {
			continue
		}
		perms, err := session.ValidatePermissions(rawPerms)
		if err != nil {
			continue
		}
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

// RbacMiddleware returns a middleware that:
//  1. Calls RequireAuth (401 if no principal).
//  2. Loads the principal's role permissions from the DB.
//  3. Derives the action from the HTTP method.
//  4. Calls session.AuthorizePerms(perms, object, action); returns 403 if denied.
func RbacMiddleware(db *sql.DB, object string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		authed := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, _ := crmctx.From(r.Context()) // guaranteed by RequireAuth above
			action := MethodToAction(r.Method)
			perms, err := LoadRolePermissions(r.Context(), db, p.TenantID, p.UserID)
			if err != nil {
				WriteInternal(w)
				return
			}
			if err := session.AuthorizePerms(perms, object, action); err != nil {
				WriteProblem(w, http.StatusForbidden, CodeForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}))
		return authed
	}
}
