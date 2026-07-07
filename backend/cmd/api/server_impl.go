package main

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/contracts/server"
	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	actstransport "github.com/gradionhq/margince/backend/internal/modules/activities/transport"
	audithistory "github.com/gradionhq/margince/backend/internal/modules/audithistory"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	dealstransport "github.com/gradionhq/margince/backend/internal/modules/deals/transport"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	organizations "github.com/gradionhq/margince/backend/internal/modules/organizations"
	orgstransport "github.com/gradionhq/margince/backend/internal/modules/organizations/transport"
	partners "github.com/gradionhq/margince/backend/internal/modules/partners"
	partnerstransport "github.com/gradionhq/margince/backend/internal/modules/partners/transport"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	relstransport "github.com/gradionhq/margince/backend/internal/modules/relationships/transport"
	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/authz"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// buildAllOperations constructs server.AllOperations (AC-D3/D10) from the
// same store/handler construction recipe registerCoreCRUD uses for its
// manual mux registration. This is compile-time interface-conformance
// wiring only — the result is not registered on the mux, and
// routes.go's own live route registration is untouched (D10).
func buildAllOperations(k *routeKit) *server.AllOperations {
	historyAuthz := authz.Authorizer(func(ctx context.Context, object, action string) error {
		p, _ := crmctx.From(ctx)
		perms, err := httpserver.LoadRolePermissions(ctx, k.db, p.TenantID, p.UserID)
		if err != nil {
			return err
		}
		return crmauth.AuthorizePerms(perms, object, action)
	})

	return server.NewAllOperations(
		server.PeopleAdapter{H: peopletransport.NewPersonHandler(people.NewPersonStore(k.db), relationships.NewRelationshipStore(k.db), deals.NewDealStore(k.db), activities.NewActivityStore(k.db), k.verifier)},
		server.OrganizationsAdapter{H: orgstransport.NewOrganizationHandler(organizations.NewOrgStore(k.db), relationships.NewRelationshipStore(k.db), deals.NewDealStore(k.db), activities.NewActivityStore(k.db), k.verifier)},
		server.DealsAdapter{H: dealstransport.NewDealHandler(deals.NewDealStore(k.db), relationships.NewRelationshipStore(k.db), activities.NewActivityStore(k.db), k.verifier)},
		server.PipelinesAdapter{
			P: dealstransport.NewPipelineHandler(deals.NewPipelineStore(k.db), deals.NewStageStore(k.db), deals.NewRollupStore(k.db)),
			S: dealstransport.NewStageHandler(deals.NewStageStore(k.db)),
		},
		server.PartnersAdapter{H: partnerstransport.NewPartnerHandler(partners.NewPartnerStore(k.db))},
		server.RelationshipsAdapter{H: relstransport.NewRelationshipHandler(relationships.NewRelationshipStore(k.db))},
		server.ActivitiesAdapter{H: actstransport.NewActivityHandler(activities.NewActivityStore(k.db))},
		server.AuditAdapter{H: audithistory.New(k.db, historyAuthz).Handler},
		*server.NewIdentityAdapter(k.db, k.sessionStore, k.passportStore),
	)
}
