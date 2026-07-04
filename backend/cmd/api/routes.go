package main

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/riverqueue/river"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	dealstransport "github.com/gradionhq/margince/backend/internal/modules/deals/transport"
	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	dealtransport "github.com/gradionhq/margince/backend/internal/modules/directory/transport"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	identitytransport "github.com/gradionhq/margince/backend/internal/modules/identity/transport"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// routeKit carries the shared wiring every route group needs: the db pool, the
// background ctx, the resolved config, the River client for enqueue closures, and
// the two middleware factories (session-only and session+RBAC) the groups compose.
type routeKit struct {
	db            *sql.DB
	ctx           context.Context
	cfg           Config
	riverClient   *river.Client[*sql.Tx]
	sessionStore  *crmauth.SessionStore
	passportStore *crmauth.PassportStore
	workspaceWrap func(http.Handler) http.Handler
	domainWrap    func(string, http.Handler) http.Handler
}

// buildMux constructs the fully-wired route mux. It builds the shared middleware
// once, then delegates to per-concern registrars so no single function carries
// the whole surface.
//
// This is the pruned platform+person surface (skeleton harvest): only
// observability, auth/workspace bootstrap, passports, roles, members, the
// /people slice, the core /deals CRUD, and the pipeline/stage read slices are
// registered here. The frozen poc's
// async/export/import, Gmail/Calendar webhook, HubSpot OAuth, approvals inbox,
// automation, and product surfaces are not wired in this tree.
func buildMux(ctx context.Context, db *sql.DB, cfg Config, riverClient *river.Client[*sql.Tx]) *http.ServeMux {
	sessionStore := crmauth.NewSessionStore(db)
	passportStore := crmauth.NewPassportStore(db)
	sessMW := httpserver.SessionMiddleware(sessionStore, passportStore)

	// workspaceWrap: reads legacy X-Workspace-ID header (dev/test) OR session context.
	workspaceWrap := func(h http.Handler) http.Handler {
		return sessMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If session middleware already set a principal, skip legacy header.
			if _, ok := crmctx.From(r.Context()); !ok {
				wsID := r.Header.Get("X-Workspace-ID")
				userID := r.Header.Get("X-User-ID")
				if wsID != "" {
					ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: wsID, UserID: userID})
					r = r.WithContext(ctx)
				}
			}
			h.ServeHTTP(w, r)
		}))
	}
	// domainWrap composes session injection + requireAuth + RBAC enforcement.
	// object is the canonical RBAC object name (e.g. "person").
	domainWrap := func(object string, h http.Handler) http.Handler {
		return workspaceWrap(httpserver.RbacMiddleware(db, object)(h))
	}

	k := &routeKit{
		db:            db,
		ctx:           ctx,
		cfg:           cfg,
		riverClient:   riverClient,
		sessionStore:  sessionStore,
		passportStore: passportStore,
		workspaceWrap: workspaceWrap,
		domainWrap:    domainWrap,
	}

	mux := http.NewServeMux()
	k.registerObservabilityAndAuth(mux)
	k.registerCoreCRUD(mux)
	return mux
}

// registerObservabilityAndAuth wires the unauthenticated metrics endpoint plus the
// auth/passport routes and the workspace/manage_members-gated role-assignment admin.
func (k *routeKit) registerObservabilityAndAuth(mux *http.ServeMux) {
	// Observability endpoint (no auth — read-only metrics).
	mux.Handle("GET /metrics", metricsHandler())

	// Auth routes (no requireAuth — login/workspace setup are unauthenticated).
	mux.Handle("POST /workspaces", identitytransport.HandleCreateWorkspace(k.db))
	mux.Handle("POST /auth/login", identitytransport.HandleLogin(k.db, k.sessionStore))
	mux.Handle("POST /auth/logout", httpserver.SessionMiddleware(k.sessionStore, k.passportStore)(identitytransport.HandleLogout(k.sessionStore)))
	mux.Handle("GET /me", k.workspaceWrap(httpserver.RequireAuth(identitytransport.HandleMe(k.db))))
	mux.Handle("POST /passports", k.workspaceWrap(httpserver.RequireAuth(identitytransport.HandleCreatePassport(k.db, k.passportStore))))
	mux.Handle("DELETE /passports/", k.workspaceWrap(httpserver.RequireAuth(identitytransport.HandleRevokePassport(k.passportStore))))

	// Role-assignment admin (E11): workspace/manage_members-gated.
	mux.Handle("GET /roles", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleListRoles(k.db))))
	mux.Handle("GET /members", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleListMembers(k.db))))
	mux.Handle("POST /members/{user_id}/roles", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleAssignRole(k.db))))
	mux.Handle("DELETE /members/{user_id}/roles/{role_key}", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleRevokeRole(k.db))))
}

// registerCoreCRUD wires the method-derived-RBAC CRUD subtree for the one kept
// core object slice: /people.
func (k *routeKit) registerCoreCRUD(mux *http.ServeMux) {
	crud := func(path, object string, h http.Handler) {
		wrapped := instrument(path, k.domainWrap(object, h))
		mux.Handle(path, wrapped)
		mux.Handle(path+"/", wrapped)
	}
	crud("/people", httpserver.ObjPerson, peopletransport.NewPersonHandler(directory.NewPersonStore(k.db)))
	crud("/organizations", httpserver.ObjOrganization, dealtransport.NewOrganizationHandler(directory.NewOrgStore(k.db)))
	dealHandler := dealtransport.NewDealHandler(directory.NewDealStore(k.db), k.db)
	crud("/deals", httpserver.ObjDeal, dealHandler)
	mux.Handle("POST /deals/{id}/advance", instrument("/deals/advance", k.domainWrap(httpserver.ObjDeal, dealHandler)))
	crud("/pipelines", httpserver.ObjPipeline, dealstransport.NewPipelineHandler(deals.NewPipelineStore(k.db), deals.NewStageStore(k.db), deals.NewRollupStore(k.db)))
	crud("/stages", httpserver.ObjStage, dealstransport.NewStageHandler(deals.NewStageStore(k.db)))
	partnerHandler := dealtransport.NewPartnerHandler(directory.NewPartnerStore(k.db))
	mux.Handle("PUT /organizations/{id}/partner", instrument("/organizations/partner", k.domainWrap(httpserver.ObjPartner, partnerHandler)))
	mux.Handle("GET /organizations/{id}/partner", instrument("/organizations/partner", k.domainWrap(httpserver.ObjPartner, partnerHandler)))
	mux.Handle("GET /partners", instrument("/partners", k.domainWrap(httpserver.ObjPartner, partnerHandler)))
	crud("/relationships", httpserver.ObjRelationship, dealtransport.NewRelationshipHandler(directory.NewRelationshipStore(k.db)))
}
