// Package domain holds the relationships module's core types.
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

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

// RelationshipListFilter holds optional predicates for List.
type RelationshipListFilter struct {
	Kind            string
	PersonID        string
	OrganizationID  string
	DealID          string
	IncludeArchived bool
}
