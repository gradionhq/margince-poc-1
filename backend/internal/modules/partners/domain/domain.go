// Package domain contains the partners module's pure domain types.
package domain

import (
	"time"

	ids "github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	prov "github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

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

// NewPartner returns a Partner with a fresh ID, applied status, version 1, and copied provenance.
func NewPartner(organizationID string, p prov.Provenance) Partner {
	return Partner{
		ID: ids.New(), OrganizationID: organizationID, CertStatus: "applied", GateMetrics: map[string]any{},
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}

// PartnerListFilter holds optional predicates for List.
type PartnerListFilter struct {
	PartnerRole string
	CertStatus  string
}
