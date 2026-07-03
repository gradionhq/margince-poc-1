// Package deals is the deals/pipeline domain module: pipeline, stage (T10),
// deal CRUD lives in modules/directory for now (T11 predates this module and
// wasn't migrated — see PR description for the discrepancy note). Implements
// no datasource.Provider seam yet; add one only when a future ticket needs it.
package deals

import "time"

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
