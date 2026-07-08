package main

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/riverqueue/river"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	actstransport "github.com/gradionhq/margince/backend/internal/modules/activities/transport"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	audithistory "github.com/gradionhq/margince/backend/internal/modules/audithistory"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	dealstransport "github.com/gradionhq/margince/backend/internal/modules/deals/transport"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	identitytransport "github.com/gradionhq/margince/backend/internal/modules/identity/transport"
	offers "github.com/gradionhq/margince/backend/internal/modules/offers"
	offerstransport "github.com/gradionhq/margince/backend/internal/modules/offers/transport"
	organizations "github.com/gradionhq/margince/backend/internal/modules/organizations"
	orgstransport "github.com/gradionhq/margince/backend/internal/modules/organizations/transport"
	partners "github.com/gradionhq/margince/backend/internal/modules/partners"
	partnerstransport "github.com/gradionhq/margince/backend/internal/modules/partners/transport"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	recordstransport "github.com/gradionhq/margince/backend/internal/modules/records/transport"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	relstransport "github.com/gradionhq/margince/backend/internal/modules/relationships/transport"
	platformauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	platformconfig "github.com/gradionhq/margince/backend/internal/platform/config"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/authz"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// routeKit carries the shared wiring every route group needs.
type routeKit struct {
	db            *sql.DB
	ctx           context.Context
	cfg           platformconfig.Config
	riverClient   *river.Client[*sql.Tx]
	sessionStore  *crmauth.SessionStore
	passportStore *crmauth.PassportStore
	verifier      *crmapprovals.DBVerifier
	workspaceWrap func(http.Handler) http.Handler
	domainWrap    func(string, http.Handler) http.Handler
}

// buildMux constructs the fully-wired route mux.
func buildMux(ctx context.Context, db *sql.DB, cfg platformconfig.Config, riverClient *river.Client[*sql.Tx]) *http.ServeMux {
	sessionStore := crmauth.NewSessionStore(db)
	passportStore := crmauth.NewPassportStore(db)
	sessMW := platformauth.SessionMiddleware(&crmauth.SessionVerifier{Sessions: sessionStore, Passports: passportStore})

	workspaceWrap := func(h http.Handler) http.Handler {
		return sessMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	domainWrap := func(object string, h http.Handler) http.Handler {
		return workspaceWrap(platformauth.RbacMiddleware(db, object)(h))
	}

	toolgate.RegisterResolver("target_stage_semantic", deals.ResolveDynamicTier)

	k := &routeKit{
		db:            db,
		ctx:           ctx,
		cfg:           cfg,
		riverClient:   riverClient,
		sessionStore:  sessionStore,
		passportStore: passportStore,
		verifier:      &crmapprovals.DBVerifier{DB: db},
		workspaceWrap: workspaceWrap,
		domainWrap:    domainWrap,
	}

	_ = buildAllOperations(k)

	mux := http.NewServeMux()
	k.registerObservabilityAndAuth(mux)
	k.registerCoreCRUD(mux)
	return mux
}

func (k *routeKit) registerObservabilityAndAuth(mux *http.ServeMux) {
	mux.Handle("GET /metrics", metricsHandler())

	mux.Handle("POST /workspaces", identitytransport.HandleCreateWorkspace(k.db))
	mux.Handle("POST /auth/login", identitytransport.HandleLogin(k.db, k.sessionStore))
	mux.Handle("POST /auth/logout", platformauth.SessionMiddleware(&crmauth.SessionVerifier{Sessions: k.sessionStore, Passports: k.passportStore})(identitytransport.HandleLogout(k.sessionStore)))
	mux.Handle("GET /me", k.workspaceWrap(platformauth.RequireAuth(identitytransport.HandleMe(k.db))))
	mux.Handle("POST /passports", k.workspaceWrap(platformauth.RequireAuth(identitytransport.HandleCreatePassport(k.db, k.passportStore))))
	mux.Handle("DELETE /passports/", k.workspaceWrap(platformauth.RequireAuth(identitytransport.HandleRevokePassport(k.passportStore))))

	mux.Handle("GET /roles", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleListRoles(k.db))))
	mux.Handle("GET /members", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleListMembers(k.db))))
	mux.Handle("POST /members/{user_id}/roles", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleAssignRole(k.db))))
	mux.Handle("DELETE /members/{user_id}/roles/{role_key}", k.workspaceWrap(identitytransport.RequireManageMembers(k.db, identitytransport.HandleRevokeRole(k.db))))
}

func (k *routeKit) registerCoreCRUD(mux *http.ServeMux) {
	crud := func(path, object string, h http.Handler) {
		wrapped := instrument(path, k.domainWrap(object, h))
		mux.Handle(path, wrapped)
		mux.Handle(path+"/", wrapped)
	}
	crud("/people", platformauth.ObjPerson, peopletransport.NewPersonHandler(people.NewPersonStore(k.db), relationships.NewRelationshipStore(k.db), deals.NewDealStore(k.db), activities.NewActivityStore(k.db), k.verifier))
	crud("/organizations", platformauth.ObjOrganization, orgstransport.NewOrganizationHandler(organizations.NewOrgStore(k.db), relationships.NewRelationshipStore(k.db), deals.NewDealStore(k.db), activities.NewActivityStore(k.db), records.NewRollupStore(k.db), k.verifier))
	dealHandler := dealstransport.NewDealHandler(deals.NewDealStore(k.db), relationships.NewRelationshipStore(k.db), activities.NewActivityStore(k.db), k.verifier)
	crud("/deals", platformauth.ObjDeal, dealHandler)
	mux.Handle("POST /deals/{id}/advance", instrument("/deals/advance", k.domainWrap(platformauth.ObjDeal, dealHandler)))
	mux.Handle("GET /deals/{id}/stakeholders", instrument("/deals/stakeholders", k.domainWrap(platformauth.ObjDeal, dealHandler)))
	crud("/pipelines", platformauth.ObjPipeline, dealstransport.NewPipelineHandler(deals.NewPipelineStore(k.db), deals.NewStageStore(k.db), deals.NewRollupStore(k.db)))
	crud("/stages", platformauth.ObjStage, dealstransport.NewStageHandler(deals.NewStageStore(k.db)))
	partnerHandler := partnerstransport.NewPartnerHandler(partners.NewPartnerStore(k.db))
	mux.Handle("PUT /organizations/{id}/partner", instrument("/organizations/partner", k.domainWrap(platformauth.ObjPartner, partnerHandler)))
	mux.Handle("GET /organizations/{id}/partner", instrument("/organizations/partner", k.domainWrap(platformauth.ObjPartner, partnerHandler)))
	mux.Handle("GET /partners", instrument("/partners", k.domainWrap(platformauth.ObjPartner, partnerHandler)))
	crud("/relationships", platformauth.ObjRelationship, relstransport.NewRelationshipHandler(relationships.NewRelationshipStore(k.db)))
	crud("/activities", platformauth.ObjActivity, actstransport.NewActivityHandler(activities.NewActivityStore(k.db)))
	crud("/record-grants", platformauth.ObjRecordGrant, relstransport.NewRecordGrantHandler(people.NewRecordGrantStore(k.db), k.db, k.verifier))
	crud("/products", platformauth.ObjProduct, offerstransport.NewProductHandler(offers.NewProductStore(k.db)))
	crud("/offer-templates", platformauth.ObjOfferTemplate, offerstransport.NewOfferTemplateHandler(offers.NewOfferTemplateStore(k.db)))
	crud("/quotas", platformauth.ObjQuota, recordstransport.NewQuotaHandler(records.NewQuotaStore(k.db)))

	historyAuthz := authz.Authorizer(func(ctx context.Context, object, action string) error {
		p, _ := crmctx.From(ctx)
		perms, err := platformauth.LoadRolePermissions(ctx, k.db, p.TenantID, p.UserID)
		if err != nil {
			return err
		}
		return crmauth.AuthorizePerms(perms, object, action)
	})
	historyHandler := audithistory.New(k.db, historyAuthz).Handler
	mux.Handle("GET /records/{entity_type}/{id}/history",
		instrument("/records/history", k.workspaceWrap(platformauth.RequireAuth(historyHandler))))
}
