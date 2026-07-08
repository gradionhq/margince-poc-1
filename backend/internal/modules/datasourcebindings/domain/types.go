// Package domain defines the minimal entity types used by the datasourcebindings module.
// Each struct carries only the fields the datasource binding reads or writes; the full
// domain types live in the entity-specific modules (people, deals, etc.).
package domain

import "time"

// Person is the minimal person record for the datasource binding.
type Person struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	FullName    string `json:"full_name"`
	Source      string `json:"source"`
	CapturedBy  string `json:"captured_by"`
}

// PersonEmailInput is one entry of a createPerson emails[] request field.
type PersonEmailInput struct {
	Email     string
	EmailType string // "work" (default) | "personal" | "other"
	IsPrimary bool
	Position  int
}

// Organization is the minimal organization record for the datasource binding.
type Organization struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
	CapturedBy  string `json:"captured_by"`
}

// OrgListFilter holds optional predicates for organization list queries.
// Zero value means no extra filters.
type OrgListFilter struct{}

// Deal is the minimal deal record for the datasource binding.
type Deal struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	PipelineID  string `json:"pipeline_id"`
	StageID     string `json:"stage_id"`
	Status      string `json:"status"` // open | won | lost
	Source      string `json:"source"`
	CapturedBy  string `json:"captured_by"`
}

// Activity is the minimal activity record for the datasource binding.
type Activity struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Kind        string    `json:"kind"`
	OccurredAt  time.Time `json:"occurred_at"`
	Source      string    `json:"source"`
	CapturedBy  string    `json:"captured_by"`
}

// Lead is the minimal lead record for the datasource binding.
type Lead struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"` // new | working | promoted | disqualified
	Source      string `json:"source"`
	CapturedBy  string `json:"captured_by"`
}
