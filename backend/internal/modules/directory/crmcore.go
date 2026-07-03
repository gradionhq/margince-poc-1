// Package crmcore is the Tier-1 domain core: person, organization, deal,
// pipeline, stage, activity, lead. Implements the datasource.Provider seam.
// Imports only Tier-0 seams (ADR-0014).
package crmcore

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// Person is a contact record (data-model §3.1).
type Person struct {
	ID                  string         `json:"id"`
	WorkspaceID         string         `json:"workspace_id"`
	FirstName           *string        `json:"first_name"`
	LastName            *string        `json:"last_name"`
	FullName            string         `json:"full_name"`
	Title               *string        `json:"title"`
	OwnerID             *string        `json:"owner_id"`
	Social              map[string]any `json:"social"`
	Address             map[string]any `json:"address"`
	MergedIntoID        *string        `json:"merged_into_id"`
	ConvertedFromLeadID *string        `json:"converted_from_lead_id"`
	Version             int64          `json:"version"`
	Source              string         `json:"source"`
	CapturedBy          string         `json:"captured_by"`
	// Provenance is kept for internal use (audit etc.); not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// Organization is a company record (data-model §4.1).
type Organization struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	DisplayName    string         `json:"display_name"`
	Website        *string        `json:"website"`
	Classification *string        `json:"classification"`
	Relevance      int            `json:"relevance"`
	OwnerID        *string        `json:"owner_id"`
	ParentOrgID    *string        `json:"parent_org_id"`
	MergedIntoID   *string        `json:"merged_into_id"`
	Social         map[string]any `json:"social"`
	Address        map[string]any `json:"address"`
	Version        int64          `json:"version"`
	Source         string         `json:"source"`
	CapturedBy     string         `json:"captured_by"`
	// Provenance is kept for internal use; not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// Pipeline is a named sales pipeline (data-model §6.1).
type Pipeline struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Name        string     `json:"name"`
	IsDefault   bool       `json:"is_default"`
	Position    int        `json:"position"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
}

// Stage is one step in a Pipeline (data-model §6.2).
type Stage struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspace_id"`
	PipelineID     string     `json:"pipeline_id"`
	Name           string     `json:"name"`
	Position       int        `json:"position"`
	Semantic       string     `json:"semantic"` // open | won | lost
	WinProbability int        `json:"win_probability"`
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
