// Package domain contains the leads module's pure domain types.
// No database/sql, no net/http — only stdlib and Tier-0 kernel imports.
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	prov "github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// Lead status lifecycle values (data-model §8, ADR-0008).
const (
	StatusNew          = "new"
	StatusWorking      = "working"
	StatusPromoted     = "promoted"
	StatusDisqualified = "disqualified"
)

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
	// Provenance is kept for internal use (audit etc.); not serialised directly.
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
}

// NewLead returns a Lead with a fresh ID, new status, version 1, and copied provenance.
func NewLead(p prov.Provenance) Lead {
	return Lead{
		ID: ids.New(), Status: StatusNew, Score: 0,
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}
