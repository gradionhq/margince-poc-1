package transport

import (
	"context"

	dealdomain "github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	reldomain "github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
)

// assembleComposite fans out to related stores for the organization-360 read,
// converting each store's domain type to the organization module's view-model.
func (h *OrganizationHandler) assembleComposite(ctx context.Context, wsID, orgID string) ([]domain.RelationshipRef, []domain.DealRef, []domain.ActivityRef, error) {
	rawRels, _, err := h.relStore.List(ctx, wsID, "", 50, reldomain.RelationshipListFilter{OrganizationID: orgID})
	if err != nil {
		return nil, nil, nil, err
	}
	rels := make([]domain.RelationshipRef, len(rawRels))
	for i, r := range rawRels {
		rels[i] = domain.RelationshipRef{
			ID:                r.ID,
			WorkspaceID:       r.WorkspaceID,
			Kind:              r.Kind,
			PersonID:          r.PersonID,
			OrganizationID:    r.OrganizationID,
			DealID:            r.DealID,
			CounterpartyOrgID: r.CounterpartyOrgID,
			Role:              r.Role,
			IsCurrentPrimary:  r.IsCurrentPrimary,
			StartedAt:         r.StartedAt,
			EndedAt:           r.EndedAt,
			Version:           r.Version,
			Source:            r.Source,
			CapturedBy:        r.CapturedBy,
			Provenance:        r.Provenance,
			CreatedAt:         r.CreatedAt,
			UpdatedAt:         r.UpdatedAt,
			ArchivedAt:        r.ArchivedAt,
		}
	}

	rawDeals, _, err := h.dealStore.ListFiltered(ctx, wsID, "", 50, dealdomain.DealListFilter{OrganizationID: orgID})
	if err != nil {
		return nil, nil, nil, err
	}
	deals := make([]domain.DealRef, len(rawDeals))
	for i, d := range rawDeals {
		deals[i] = domain.DealRef{
			ID:                d.ID,
			WorkspaceID:       d.WorkspaceID,
			Name:              d.Name,
			AmountMinor:       d.AmountMinor,
			Currency:          d.Currency,
			FxRateToBase:      d.FxRateToBase,
			FxRateDate:        d.FxRateDate,
			PipelineID:        d.PipelineID,
			StageID:           d.StageID,
			OrganizationID:    d.OrganizationID,
			OwnerID:           d.OwnerID,
			PartnerOrgID:      d.PartnerOrgID,
			Status:            d.Status,
			LostReason:        d.LostReason,
			ExpectedCloseDate: d.ExpectedCloseDate,
			ClosedAt:          d.ClosedAt,
			ForecastCategory:  d.ForecastCategory,
			WaitUntil:         d.WaitUntil,
			LastActivityAt:    d.LastActivityAt,
			Stalled:           d.Stalled,
			StageEnteredAt:    d.StageEnteredAt,
			StakeholderCount:  d.StakeholderCount,
			Version:           d.Version,
			Source:            d.Source,
			CapturedBy:        d.CapturedBy,
			Provenance:        d.Provenance,
			CreatedAt:         d.CreatedAt,
			UpdatedAt:         d.UpdatedAt,
			ArchivedAt:        d.ArchivedAt,
		}
	}

	rawActs, _, err := h.activityStore.List(ctx, wsID, "organization", orgID, "", 50)
	if err != nil {
		return nil, nil, nil, err
	}
	acts := make([]domain.ActivityRef, len(rawActs))
	for i, a := range rawActs {
		acts[i] = domain.ActivityRef{ID: a.ID, Kind: a.Kind, Subject: a.Subject, OccurredAt: a.OccurredAt}
	}
	return rels, deals, acts, nil
}
