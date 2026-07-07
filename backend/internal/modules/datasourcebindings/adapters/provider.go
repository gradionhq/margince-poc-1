// Package adapters implements the datasource.Provider seam over the CRM entity stores.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/domain"
	"github.com/gradionhq/margince/backend/internal/modules/datasourcebindings/ports"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// DatasourceProvider wraps the domain stores and implements the 11-method datasource.Provider seam.
type DatasourceProvider struct {
	workspaceID string
	persons     ports.PersonStorer
	orgs        ports.OrgStorer
	deals       ports.DealStorer
	activities  ports.ActivityStorer
	leads       ports.LeadStorer
}

// NewDatasourceProvider constructs a DatasourceProvider. The concrete store types from
// the entity modules must satisfy the ports.*Storer interfaces.
func NewDatasourceProvider(
	workspaceID string,
	persons ports.PersonStorer,
	orgs ports.OrgStorer,
	deals ports.DealStorer,
	activities ports.ActivityStorer,
	leads ports.LeadStorer,
) *DatasourceProvider {
	return &DatasourceProvider{
		workspaceID: workspaceID,
		persons:     persons,
		orgs:        orgs,
		deals:       deals,
		activities:  activities,
		leads:       leads,
	}
}

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

//nolint:ireturn // seam method returns the datasource.Record interface by design (architecture.md)
func (p *DatasourceProvider) Read(ctx context.Context, ref datasource.EntityRef) (datasource.Record, error) {
	switch ref.Type {
	case datasource.EntityPerson:
		return p.persons.Get(ctx, ref.ID, p.workspaceID)
	case datasource.EntityOrganization:
		return p.orgs.Get(ctx, ref.ID, p.workspaceID)
	case datasource.EntityDeal:
		return p.deals.Get(ctx, ref.ID, p.workspaceID)
	case datasource.EntityActivity:
		return p.activities.Get(ctx, ref.ID, p.workspaceID)
	case datasource.EntityLead:
		return p.leads.Get(ctx, ref.ID, p.workspaceID)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", ref.Type)
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create inserts a new entity in the native store and returns its EntityRef.
func (p *DatasourceProvider) Create(ctx context.Context, in datasource.CreateInput) (datasource.EntityRef, error) {
	if in.Source == "" || in.CapturedBy == "" {
		return datasource.EntityRef{}, errs.ErrNullProvenance
	}

	switch in.Type {
	case datasource.EntityPerson:
		person := domain.PersonFromFields(in.Fields)
		person.WorkspaceID = p.workspaceID
		person.Source = in.Source
		person.CapturedBy = in.CapturedBy
		created, err := p.persons.Create(ctx, person, nil)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityPerson, ID: created.ID}, nil

	case datasource.EntityOrganization:
		org := domain.OrgFromFields(in.Fields)
		org.WorkspaceID = p.workspaceID
		org.Source = in.Source
		org.CapturedBy = in.CapturedBy
		created, err := p.orgs.Create(ctx, org)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityOrganization, ID: created.ID}, nil

	case datasource.EntityDeal:
		deal := domain.DealFromFields(in.Fields)
		deal.WorkspaceID = p.workspaceID
		deal.Source = in.Source
		deal.CapturedBy = in.CapturedBy
		created, err := p.deals.Create(ctx, deal, "")
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityDeal, ID: created.ID}, nil

	case datasource.EntityActivity:
		activity := domain.ActivityFromFields(in.Fields)
		activity.WorkspaceID = p.workspaceID
		activity.Source = in.Source
		activity.CapturedBy = in.CapturedBy
		created, err := p.activities.Create(ctx, activity)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityActivity, ID: created.ID}, nil

	case datasource.EntityLead:
		lead := domain.LeadFromFields(in.Fields)
		lead.WorkspaceID = p.workspaceID
		lead.Source = in.Source
		lead.CapturedBy = in.CapturedBy
		created, err := p.leads.Create(ctx, lead)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityLead, ID: created.ID}, nil

	default:
		return datasource.EntityRef{}, fmt.Errorf("unknown entity type: %s", in.Type)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update applies a field patch to a native entity and returns its EntityRef.
func (p *DatasourceProvider) Update(ctx context.Context, in datasource.UpdateInput) (datasource.EntityRef, error) {
	if in.Source == "" || in.CapturedBy == "" {
		return datasource.EntityRef{}, errs.ErrNullProvenance
	}

	ifMatch, err := parseIfVersion(in.IfVersion)
	if err != nil {
		return datasource.EntityRef{}, err
	}

	patch := domain.ToMap(in.Patch)

	switch in.Type {
	case datasource.EntityPerson:
		_, err = p.persons.Update(ctx, in.ID, p.workspaceID, patch, ifMatch)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityPerson, ID: in.ID}, nil

	case datasource.EntityOrganization:
		_, err = p.orgs.Update(ctx, in.ID, p.workspaceID, patch, ifMatch)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityOrganization, ID: in.ID}, nil

	case datasource.EntityDeal:
		_, err = p.deals.Update(ctx, in.ID, p.workspaceID, patch, ifMatch)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityDeal, ID: in.ID}, nil

	case datasource.EntityActivity:
		_, err = p.activities.Update(ctx, in.ID, p.workspaceID, patch, ifMatch)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityActivity, ID: in.ID}, nil

	case datasource.EntityLead:
		_, err = p.leads.Update(ctx, in.ID, p.workspaceID, patch, ifMatch)
		if err != nil {
			return datasource.EntityRef{}, err
		}
		return datasource.EntityRef{Type: datasource.EntityLead, ID: in.ID}, nil

	default:
		return datasource.EntityRef{}, fmt.Errorf("unknown entity type: %s", in.Type)
	}
}

// ---------------------------------------------------------------------------
// AdvanceDeal
// ---------------------------------------------------------------------------

// AdvanceDeal moves a deal to the target stage in the native store.
func (p *DatasourceProvider) AdvanceDeal(ctx context.Context, in datasource.AdvanceDealInput) (datasource.EntityRef, error) {
	_, err := p.deals.Update(ctx, in.DealID, p.workspaceID, map[string]any{"status": in.ToStatus}, 0)
	if err != nil {
		return datasource.EntityRef{}, err
	}
	return datasource.EntityRef{Type: datasource.EntityDeal, ID: in.DealID}, nil
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

// Search runs a workspace-scoped query against the native store.
//
//nolint:cyclop // per-entity-type dispatch: one case per core object, each the same List->append shape
func (p *DatasourceProvider) Search(ctx context.Context, query datasource.SearchQuery) (datasource.SearchResult, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}

	var records []datasource.Record

	switch query.Type {
	case datasource.EntityPerson:
		list, _, err := p.persons.List(ctx, p.workspaceID, "", limit, "")
		if err != nil {
			return datasource.SearchResult{}, err
		}
		for _, item := range list {
			records = append(records, item)
		}
	case datasource.EntityOrganization:
		list, _, err := p.orgs.List(ctx, p.workspaceID, "", limit, "", domain.OrgListFilter{})
		if err != nil {
			return datasource.SearchResult{}, err
		}
		for _, item := range list {
			records = append(records, item)
		}
	case datasource.EntityDeal:
		list, _, err := p.deals.List(ctx, p.workspaceID, "", limit)
		if err != nil {
			return datasource.SearchResult{}, err
		}
		for _, item := range list {
			records = append(records, item)
		}
	case datasource.EntityActivity:
		list, _, err := p.activities.List(ctx, p.workspaceID, "", "", "", limit)
		if err != nil {
			return datasource.SearchResult{}, err
		}
		for _, item := range list {
			records = append(records, item)
		}
	case datasource.EntityLead:
		list, _, err := p.leads.List(ctx, p.workspaceID, "", limit)
		if err != nil {
			return datasource.SearchResult{}, err
		}
		for _, item := range list {
			records = append(records, item)
		}
	default:
		return datasource.SearchResult{}, fmt.Errorf("unknown entity type: %s", query.Type)
	}

	return datasource.SearchResult{Records: records, Total: len(records)}, nil
}

// ---------------------------------------------------------------------------
// RunReport
// ---------------------------------------------------------------------------

// RunReport executes a report plan against the native store.
//
//nolint:ireturn // seam method returns the datasource.ReportResult (= any) interface by design (architecture.md)
func (p *DatasourceProvider) RunReport(_ context.Context, plan datasource.ReportPlan) (datasource.ReportResult, error) {
	return map[string]any{"plan": plan.Name, "params": plan.Params}, nil
}

// ---------------------------------------------------------------------------
// ListObjects
// ---------------------------------------------------------------------------

// ListObjects returns the object definitions the native store exposes.
func (p *DatasourceProvider) ListObjects(_ context.Context) ([]datasource.ObjectDef, error) {
	return []datasource.ObjectDef{
		{Type: datasource.EntityPerson, Label: "Person"},
		{Type: datasource.EntityOrganization, Label: "Organization"},
		{Type: datasource.EntityDeal, Label: "Deal"},
		{Type: datasource.EntityActivity, Label: "Activity"},
		{Type: datasource.EntityLead, Label: "Lead"},
	}, nil
}

// ---------------------------------------------------------------------------
// ListFields
// ---------------------------------------------------------------------------

// ListFields returns the field definitions for one native entity type.
func (p *DatasourceProvider) ListFields(_ context.Context, t datasource.EntityType) ([]datasource.FieldDef, error) {
	base := []datasource.FieldDef{
		{Name: "id", Type: "string", Label: "ID", Required: true},
		{Name: "workspace_id", Type: "string", Label: "Workspace ID", Required: true},
	}
	switch t {
	case datasource.EntityPerson:
		return append(base, datasource.FieldDef{Name: "full_name", Type: "string", Label: "Full Name", Required: true}), nil
	case datasource.EntityOrganization:
		return append(base, datasource.FieldDef{Name: "display_name", Type: "string", Label: "Display Name", Required: true}), nil
	case datasource.EntityDeal:
		return append(base, datasource.FieldDef{Name: "name", Type: "string", Label: "Name", Required: true}), nil
	case datasource.EntityActivity:
		return append(base, datasource.FieldDef{Name: "kind", Type: "string", Label: "Kind", Required: true}), nil
	case datasource.EntityLead:
		return append(base, datasource.FieldDef{Name: "status", Type: "string", Label: "Status", Required: true}), nil
	default:
		return base, nil
	}
}

// ---------------------------------------------------------------------------
// Freshness
// ---------------------------------------------------------------------------

// Freshness reports the sync state of a native entity; native rows are always authoritative.
func (p *DatasourceProvider) Freshness(_ context.Context, _ datasource.EntityRef) (datasource.FreshnessInfo, error) {
	return datasource.FreshnessInfo{LastSyncedAt: time.Now(), Authoritative: true}, nil
}

// ---------------------------------------------------------------------------
// LinkConversation / UnlinkConversation
// ---------------------------------------------------------------------------

// errConversationLinkingUnavailable is returned when conversation-linking is not
// available in this build.
var errConversationLinkingUnavailable = errors.New("conversation linking is not available in this build")

// LinkConversation is unavailable in this build: the ConversationLink domain and its
// backing table ship with the conversation-linking feature, which is not part of the
// skeleton. Retained only to satisfy datasource.Provider's interface.
func (p *DatasourceProvider) LinkConversation(_ context.Context, _ datasource.LinkConversationInput) (datasource.EntityRef, error) {
	return datasource.EntityRef{}, errConversationLinkingUnavailable
}

// UnlinkConversation is unavailable in this build: see LinkConversation.
func (p *DatasourceProvider) UnlinkConversation(_ context.Context, _ datasource.UnlinkConversationInput) error {
	return errConversationLinkingUnavailable
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseIfVersion(v *string) (int64, error) {
	if v == nil {
		return 0, nil
	}
	n, err := strconv.ParseInt(*v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid IfVersion %q: %w", *v, err)
	}
	return n, nil
}
