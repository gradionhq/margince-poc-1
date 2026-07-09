// Package domain holds the organizations module's core domain types (WS-E-a).
// Extracted from modules/directory/crmcore.go and modules/directory/org_strength.go.
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/dedupe"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

// Organization is a company record (data-model §4.1).
type Organization struct {
	ID             string               `json:"id"`
	WorkspaceID    string               `json:"workspace_id"`
	DisplayName    string               `json:"display_name"`
	Website        *string              `json:"website"`
	Classification *string              `json:"classification"`
	Relevance      int                  `json:"relevance"`
	OwnerID        *string              `json:"owner_id"`
	ParentOrgID    *string              `json:"parent_org_id"`
	MergedIntoID   *string              `json:"merged_into_id"`
	Social         map[string]any       `json:"social"`
	Address        map[string]any       `json:"address"`
	Domains        []OrganizationDomain `json:"domains"`
	ContactCount   int                  `json:"contact_count"`
	OpenDealCount  int                  `json:"open_deal_count"`
	Strength       *OrgStrengthBlock    `json:"org_strength"`
	Version        int64                `json:"version"`
	Source         string               `json:"source"`
	CapturedBy     string               `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
	// ReviewFlag is PO-AC-19's non-blocking fuzzy-dedupe flag (PO-F-2
	// name-only tier), computed fresh on every Create call, never persisted.
	ReviewFlag    *dedupe.ReviewFlag `json:"dedupe_review,omitempty"`
	Relationships []RelationshipRef  `json:"relationships,omitempty"`
	Deals         []DealRef          `json:"deals,omitempty"`
	Activities    []ActivityRef      `json:"activities,omitempty"`
	CustomFields  map[string]any     `json:"-"`
}

// OrganizationDomain is a normalized domain owned by an organization
// (data-model §4.2). The backing table (`organization_domain`, migration
// 000006) has no source/captured_by columns - this type only carries the
// columns that exist; do not add provenance columns here (out of scope).
type OrganizationDomain struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	Domain         string     `json:"domain"`
	IsPrimary      bool       `json:"is_primary"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ArchivedAt     *time.Time `json:"archived_at"`
}

// OrgStrengthBlock is the wire shape of PO-N-ORGSTRENGTH's org roll-up.
type OrgStrengthBlock struct {
	Score         int    `json:"score"`
	Bucket        string `json:"bucket"`
	TopPersonID   string `json:"top_person_id"`
	TopPersonName string `json:"top_person_name"`
}

// NewOrganization returns an Organization with a fresh ID, version 1, and copied provenance.
func NewOrganization(name string, p prov.Provenance) Organization {
	return Organization{
		ID: ids.New(), DisplayName: name, Social: map[string]any{},
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// OrgListFilter holds optional predicates for OrgStore.List. Zero value = no extra filters.
type OrgListFilter struct {
	Classification string
	RelevanceGTE   *int
	Domain         string
	OwnerID        string
	CustomFilters  map[string]string
}

// OrgStrengthInput is one person's contribution to PO-N-ORGSTRENGTH's org
// roll-up. Strength is nil for a no-signal-yet person, so it is excluded from
// the max rather than treated as zero. LastInteraction is only used for
// tie-breaking among people with a computed strength.
type OrgStrengthInput struct {
	PersonID        string
	Strength        *strength.Result
	LastInteraction time.Time
}

// OrgStrength implements PO-N-ORGSTRENGTH: plain max over the org's people's
// strengths, with no cap and no normalization. Ties resolve by most recent
// LastInteraction, then lowest PersonID. hasSignal is false when every person is
// no-signal-yet.
func OrgStrength(people []OrgStrengthInput) (score int, topPersonID string, hasSignal bool) {
	var best *OrgStrengthInput
	for i := range people {
		p := &people[i]
		if p.Strength == nil {
			continue
		}
		switch {
		case best == nil:
			best = p
		case p.Strength.Score > best.Strength.Score:
			best = p
		case p.Strength.Score == best.Strength.Score:
			if p.LastInteraction.After(best.LastInteraction) {
				best = p
			} else if p.LastInteraction.Equal(best.LastInteraction) && p.PersonID < best.PersonID {
				best = p
			}
		}
	}
	if best == nil {
		return 0, "", false
	}
	return best.Strength.Score, best.PersonID, true
}

// RelationshipRef is the view-model for a relationship edge in an organization
// composite read. Carries all Relationship fields (data-model §5.1).
type RelationshipRef struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	Kind              string     `json:"kind"`
	PersonID          *string    `json:"person_id"`
	OrganizationID    *string    `json:"organization_id"`
	DealID            *string    `json:"deal_id"`
	CounterpartyOrgID *string    `json:"counterparty_org_id"`
	Role              *string    `json:"role"`
	IsCurrentPrimary  bool       `json:"is_current_primary"`
	StartedAt         *time.Time `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at"`
	Version           int64      `json:"version"`
	Source            string     `json:"source"`
	CapturedBy        string     `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// DealRef is the view-model for a deal in an organization composite read.
// Carries all Deal fields (data-model §6.3).
type DealRef struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	Name              string     `json:"name"`
	AmountMinor       *int64     `json:"amount_minor"`
	Currency          *string    `json:"currency"`
	FxRateToBase      *float64   `json:"fx_rate_to_base"`
	FxRateDate        *time.Time `json:"fx_rate_date"`
	PipelineID        string     `json:"pipeline_id"`
	StageID           string     `json:"stage_id"`
	OrganizationID    *string    `json:"organization_id"`
	OwnerID           *string    `json:"owner_id"`
	PartnerOrgID      *string    `json:"partner_org_id"`
	Status            string     `json:"status"` // open | won | lost
	LostReason        *string    `json:"lost_reason"`
	ExpectedCloseDate *time.Time `json:"expected_close_date"`
	ClosedAt          *time.Time `json:"closed_at"`
	ForecastCategory  *string    `json:"forecast_category"`
	WaitUntil         *time.Time `json:"wait_until"`
	LastActivityAt    *time.Time `json:"last_activity_at"`
	Stalled           bool       `json:"stalled"`
	StageEnteredAt    *time.Time `json:"stage_entered_at"`
	StakeholderCount  int        `json:"stakeholder_count"`
	Version           int64      `json:"version"`
	Source            string     `json:"source"`
	CapturedBy        string     `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// ActivityRef is a lightweight activity identity reference for composite reads.
type ActivityRef struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Subject    *string   `json:"subject"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ToActivityRef narrows a full activity to the fields composite reads carry.
func ToActivityRef(id, kind string, subject *string, occurredAt time.Time) ActivityRef {
	return ActivityRef{ID: id, Kind: kind, Subject: subject, OccurredAt: occurredAt}
}
