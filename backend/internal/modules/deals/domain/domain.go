// Package domain contains the deals module's pure domain types.
package domain

import "time"

// Pipeline is a named sales pipeline (data-model §6.1).
type Pipeline struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	Position    int    `json:"position"`
	// Stages is populated by getPipeline (single-pipeline read) with the
	// pipeline's ordered stages, per the crm.yaml Pipeline schema's "embedded
	// stages on GET" contract. listPipelines leaves it nil/omitted — embedding
	// on a list response would be N+1-query-expensive and isn't asked for by
	// the contract.
	Stages     []Stage    `json:"stages,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"`
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
