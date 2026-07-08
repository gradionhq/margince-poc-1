// Package auth holds the shared HTTP middleware stack (session/workspace/RBAC)
// and request-logging middleware. Extracted from internal/platform/httpserver
// by WS-E-d (Task 8, AC-E4); behavior is unchanged.
package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/platform/logger"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

const keyStatus = "status"

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
			"trace_id", logger.TraceID(r.Context()),
		)
	})
}

// SessionMiddleware reads crm_session cookie → LookupSession → crmctx injection.
// Also propagates W3C traceparent header into the ctx trace (mints a fresh trace if absent).
// Falls through silently if no cookie (RequireAuth enforces presence).
func SessionMiddleware(v session.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if tid, sid, ok := logger.ParseTraceparent(r.Header.Get("traceparent")); ok {
				ctx = logger.WithTrace(ctx, tid, sid)
			} else {
				ctx = logger.WithTrace(ctx, logger.NewTraceID(), logger.NewSpanID())
			}

			if cookie, err := r.Cookie(session.CookieName); err == nil && cookie.Value != "" {
				if p, ok := v.LookupSession(ctx, cookie.Value); ok {
					ctx = crmctx.With(ctx, p)
				}
			}
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
			httpserver.WriteProblem(w, http.StatusUnauthorized, httpserver.CodeUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Canonical RBAC action names, shared by MethodToAction and the explicit gates.
const (
	ActionRead    = "read"
	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionArchive = "archive"
)

// Canonical RBAC object names.
const (
	ObjPerson        = "person"
	ObjDeal          = "deal"
	ObjOrganization  = "organization"
	ObjPipeline      = "pipeline"
	ObjStage         = "stage"
	ObjPartner       = "partner"
	ObjRelationship  = "relationship"
	ObjActivity      = "activity"
	ObjRecordGrant   = "record_grant"
	ObjCustomField   = "custom_field"
	ObjProduct       = "product"
	ObjOfferTemplate = "offer_template"
	ObjAttachment    = "attachment"
	ObjOffer         = "offer"
	ObjQuota         = "quota"
)

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
			p, _ := crmctx.From(r.Context())
			action := MethodToAction(r.Method)
			perms, err := LoadRolePermissions(r.Context(), db, p.TenantID, p.UserID)
			if err != nil {
				httpserver.WriteInternal(w)
				return
			}
			if err := session.AuthorizePerms(perms, object, action); err != nil {
				httpserver.WriteProblem(w, http.StatusForbidden, httpserver.CodeForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}))
		return authed
	}
}
