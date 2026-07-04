// Package crmcore is the Tier-1 domain core: person, organization, deal,
// activity, lead. Implements the datasource.Provider seam.
// Imports only Tier-0 seams (ADR-0014).
package crmcore

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// PersonEmailInput is one entry of createPerson's emails[] request field
// (crm.yaml CreatePersonRequest.emails).
type PersonEmailInput struct {
	Email     string
	EmailType string // "work" (default) | "personal" | "other"
	IsPrimary bool
	Position  int
}

// Person is a contact record (data-model §3.1).
type Person struct {
	ID                  string          `json:"id"`
	WorkspaceID         string          `json:"workspace_id"`
	FirstName           *string         `json:"first_name"`
	LastName            *string         `json:"last_name"`
	FullName            string          `json:"full_name"`
	Title               *string         `json:"title"`
	OwnerID             *string         `json:"owner_id"`
	Social              map[string]any  `json:"social"`
	Address             map[string]any  `json:"address"`
	MergedIntoID        *string         `json:"merged_into_id"`
	ConvertedFromLeadID *string         `json:"converted_from_lead_id"`
	Version             int64           `json:"version"`
	Strength            *PersonStrength `json:"strength"`
	LastActivityAt      *time.Time      `json:"last_activity_at"`
	Source              string          `json:"source"`
	CapturedBy          string          `json:"captured_by"`
	// Provenance is kept for internal use (audit etc.); not serialised directly.
	Provenance    prov.Provenance `json:"-"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ArchivedAt    *time.Time      `json:"archived_at"`
	Relationships []Relationship  `json:"relationships,omitempty"`
	Deals         []Deal          `json:"deals,omitempty"`
	Activities    []ActivityRef   `json:"activities,omitempty"`
}

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
	Provenance    prov.Provenance `json:"-"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ArchivedAt    *time.Time      `json:"archived_at"`
	Relationships []Relationship  `json:"relationships,omitempty"`
	Deals         []Deal          `json:"deals,omitempty"`
	Activities    []ActivityRef   `json:"activities,omitempty"`
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

// Deal is a sales opportunity (data-model §6.3).
type Deal struct {
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

// Partner is the 1:1 partner-program extension of an organization.
type Partner struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	OrganizationID string         `json:"organization_id"`
	CertStatus     string         `json:"cert_status"`
	PartnerRole    *string        `json:"partner_role"`
	MarginTier     *string        `json:"margin_tier"`
	GateMetrics    map[string]any `json:"gate_metrics"`
	CertifiedStaff int            `json:"certified_staff"`
	RetentionRate  *float64       `json:"retention_rate"`
	JoinedAt       *time.Time     `json:"joined_at"`
	RenewsAt       *time.Time     `json:"renews_at"`
	Version        int64          `json:"version"`
	Source         string         `json:"source"`
	CapturedBy     string         `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// Relationship is the typed edge (employment, deal_stakeholder, or the
// partner-org kinds). Mirrors `relationship` (PO-DDL-7). T08 owns the write
// surface for employment/deal_stakeholder only; the partner-org kinds
// (partner_of/referred_by/co_sell_with) are readable via List but T15/A41
// owns their write surface (deal.partner_org_id is the pointer, not this edge).
type Relationship struct {
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

// Activity is a timeline event linked to people/orgs/deals (data-model §7).
type Activity struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Kind            string     `json:"kind"` // email | call | meeting | note | task | whatsapp | telegram
	Subject         *string    `json:"subject"`
	Body            *string    `json:"body"`
	OccurredAt      time.Time  `json:"occurred_at"`
	DueAt           *time.Time `json:"due_at"`
	AssigneeID      *string    `json:"assignee_id"`
	RemindAt        *time.Time `json:"remind_at"`
	IsDone          bool       `json:"is_done"`
	DoneAt          *time.Time `json:"done_at"`
	DurationSeconds *int       `json:"duration_seconds"`
	Direction       *string    `json:"direction"` // inbound | outbound
	MeetingStatus   *string    `json:"meeting_status"`
	SourceSystem    *string    `json:"source_system"`
	SourceID        *string    `json:"source_id"`
	TranscriptRef   *string    `json:"transcript_ref"`
	Version         int64      `json:"version"`
	Source          string     `json:"source"`
	CapturedBy      string     `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// Lead is a thin, segregated prospect record (data-model §8, ADR-0008).
type Lead struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspace_id"`
	FullName         *string    `json:"full_name"`
	Email            *string    `json:"email"`
	Title            *string    `json:"title"`
	CompanyName      *string    `json:"company_name"`
	CandidateOrgKey  *string    `json:"candidate_org_key"`
	Status           string     `json:"status"` // new | working | promoted | disqualified
	Score            int        `json:"score"`
	OwnerID          *string    `json:"owner_id"`
	PromotedPersonID *string    `json:"promoted_person_id"`
	PromotedAt       *time.Time `json:"promoted_at"`
	SourceSystem     *string    `json:"source_system"`
	SourceID         *string    `json:"source_id"`
	Version          int64      `json:"version"`
	Source           string     `json:"source"`
	CapturedBy       string     `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// Constructors

// NewPerson returns a Person with a fresh ID, version 1, and copied provenance.
func NewPerson(fullName string, p prov.Provenance) Person {
	return Person{
		ID: ids.New(), FullName: fullName, Social: map[string]any{},
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// OrgStrengthBlock is the wire shape of PO-N-ORGSTRENGTH's org roll-up.
type OrgStrengthBlock struct {
	Score         int    `json:"score"`
	Bucket        string `json:"bucket"`
	TopPersonID   string `json:"top_person_id"`
	TopPersonName string `json:"top_person_name"`
}

// PersonStrength is the wire shape of PO-EXT-1's relationship-strength block.
type PersonStrength struct {
	Score            int     `json:"score"`
	Bucket           string  `json:"bucket"`
	Recency          float64 `json:"recency"`
	Frequency        float64 `json:"frequency"`
	Reciprocity      float64 `json:"reciprocity"`
	NoRecentActivity bool    `json:"no_recent_activity,omitempty"`
}

// personStrengthFrom converts a StrengthResult into the wire PersonStrength,
// or nil for the no-signal-yet case.
func personStrengthFrom(r StrengthResult) *PersonStrength {
	if r.NoSignalYet {
		return nil
	}
	return &PersonStrength{
		Score: r.Score, Bucket: r.Bucket,
		Recency: r.Recency, Frequency: r.Frequency, Reciprocity: r.Reciprocity,
		NoRecentActivity: r.NoRecentActivity,
	}
}

// NewOrganization returns an Organization with a fresh ID, version 1, and copied provenance.
func NewOrganization(name string, p prov.Provenance) Organization {
	return Organization{
		ID: ids.New(), DisplayName: name, Social: map[string]any{},
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// NewDeal returns a Deal with a fresh ID, open status, version 1, and copied provenance.
func NewDeal(name, pipelineID, stageID string, p prov.Provenance) Deal {
	return Deal{
		ID: ids.New(), Name: name, PipelineID: pipelineID, StageID: stageID, Status: statusOpen,
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// NewPartner returns a Partner with a fresh ID, applied status, version 1, and copied provenance.
func NewPartner(organizationID string, p prov.Provenance) Partner {
	return Partner{
		ID: ids.New(), OrganizationID: organizationID, CertStatus: "applied", GateMetrics: map[string]any{},
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// NewActivity returns an Activity with a fresh ID, version 1, and copied provenance.
func NewActivity(kind string, p prov.Provenance) Activity {
	now := time.Now().UTC()
	return Activity{
		ID: ids.New(), Kind: kind, OccurredAt: now,
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// NewLead returns a Lead with a fresh ID, new status, version 1, and copied provenance.
func NewLead(p prov.Provenance) Lead {
	return Lead{
		ID: ids.New(), Status: statusNew, Score: 0,
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}
