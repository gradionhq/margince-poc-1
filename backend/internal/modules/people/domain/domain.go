// Package domain holds the people module's pure domain types.
// Imports only Tier-0 kernel seams and sibling module domain packages
// (one-directional, no circular deps — ADR-0014).
package domain

import (
	"time"

	activitiesdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	dealsdomain "github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	relationshipsdomain "github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/dedupe"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

// PersonEmailInput is one entry of createPerson's emails[] request field
// (crm.yaml CreatePersonRequest.emails).
type PersonEmailInput struct {
	Email     string
	EmailType string // "work" (default) | "personal" | "other"
	IsPrimary bool
	Position  int
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

// PersonStrengthFrom converts a strength.StrengthResult into the wire
// PersonStrength, or nil for the no-signal-yet case.
func PersonStrengthFrom(r strength.StrengthResult) *PersonStrength {
	if r.NoSignalYet {
		return nil
	}
	return &PersonStrength{
		Score: r.Score, Bucket: r.Bucket,
		Recency: r.Recency, Frequency: r.Frequency, Reciprocity: r.Reciprocity,
		NoRecentActivity: r.NoRecentActivity,
	}
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
	Provenance prov.Provenance `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
	// ReviewFlag is PO-AC-19's non-blocking fuzzy-dedupe flag, computed fresh on
	// every Create call and never persisted (dedupe.DedupeReviewFlag).
	ReviewFlag    *dedupe.DedupeReviewFlag           `json:"dedupe_review,omitempty"`
	Relationships []relationshipsdomain.Relationship `json:"relationships,omitempty"`
	Deals         []dealsdomain.Deal                 `json:"deals,omitempty"`
	Activities    []activitiesdomain.ActivityRef      `json:"activities,omitempty"`
}

// NewPerson returns a Person with a fresh ID, version 1, and copied provenance.
func NewPerson(fullName string, p prov.Provenance) Person {
	return Person{
		ID: ids.New(), FullName: fullName, Social: map[string]any{},
		Provenance: p, Source: p.Source, CapturedBy: p.CapturedBy, Version: 1,
	}
}
