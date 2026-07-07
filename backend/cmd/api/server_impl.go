package main

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/contracts/server"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	dealstransport "github.com/gradionhq/margince/backend/internal/modules/deals/transport"
	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	dealtransport "github.com/gradionhq/margince/backend/internal/modules/directory/transport"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// buildAllOperations constructs server.AllOperations (AC-D3/D10) from the
// same store/handler construction recipe registerCoreCRUD uses for its
// manual mux registration. This is compile-time interface-conformance
// wiring only — the result is not registered on the mux, and
// routes.go's own live route registration is untouched (D10).
func buildAllOperations(k *routeKit) *server.AllOperations {
	historyAuthz := func(ctx context.Context, object, action string) error {
		p, _ := crmctx.From(ctx)
		perms, err := httpserver.LoadRolePermissions(ctx, k.db, p.TenantID, p.UserID)
		if err != nil {
			return err
		}
		return crmauth.AuthorizePerms(perms, object, action)
	}

	return server.NewAllOperations(
		server.PeopleAdapter{H: peopletransport.NewPersonHandler(directory.NewPersonStore(k.db), directory.NewRelationshipStore(k.db), directory.NewDealStore(k.db), directory.NewActivityStore(k.db), k.db)},
		server.OrganizationsAdapter{H: dealtransport.NewOrganizationHandler(directory.NewOrgStore(k.db), directory.NewRelationshipStore(k.db), directory.NewDealStore(k.db), directory.NewActivityStore(k.db), k.db)},
		server.DealsAdapter{H: dealtransport.NewDealHandler(directory.NewDealStore(k.db), directory.NewRelationshipStore(k.db), directory.NewActivityStore(k.db), k.db)},
		server.PipelinesAdapter{
			P: dealstransport.NewPipelineHandler(deals.NewPipelineStore(k.db), deals.NewStageStore(k.db), deals.NewRollupStore(k.db)),
			S: dealstransport.NewStageHandler(deals.NewStageStore(k.db)),
		},
		server.PartnersAdapter{H: dealtransport.NewPartnerHandler(directory.NewPartnerStore(k.db))},
		server.RelationshipsAdapter{H: dealtransport.NewRelationshipHandler(directory.NewRelationshipStore(k.db))},
		server.ActivitiesAdapter{H: dealtransport.NewActivityHandler(directory.NewActivityStore(k.db))},
		server.AuditAdapter{H: directory.NewHistoryHandler(directory.NewAuditHistoryReader(k.db), historyAuthz)},
		*server.NewIdentityAdapter(k.db, k.sessionStore, k.passportStore),
	)
}
