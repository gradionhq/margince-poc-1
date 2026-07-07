package server

import (
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	dealstransport "github.com/gradionhq/margince/backend/internal/modules/deals/transport"
)

// PipelinesAdapter implements the Pipelines tag's slice of
// types.ServerInterface — covering both /pipelines and /stages — by
// delegating to the real PipelineHandler and StageHandler cmd/api/routes.go
// already wires for those paths.
type PipelinesAdapter struct {
	P *dealstransport.PipelineHandler
	S *dealstransport.StageHandler
}

// ListPipelines delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) ListPipelines(w http.ResponseWriter, r *http.Request, params types.ListPipelinesParams) {
	a.P.ServeHTTP(w, r)
}

// CreatePipeline delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) CreatePipeline(w http.ResponseWriter, r *http.Request, params types.CreatePipelineParams) {
	a.P.ServeHTTP(w, r)
}

// GetPipeline delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) GetPipeline(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.P.ServeHTTP(w, r)
}

// UpdatePipeline delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) UpdatePipeline(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdatePipelineParams) {
	a.P.ServeHTTP(w, r)
}

// ArchivePipeline delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) ArchivePipeline(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.P.ServeHTTP(w, r)
}

// GetPipelineRollup delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) GetPipelineRollup(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.P.ServeHTTP(w, r)
}

// ListStages delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) ListStages(w http.ResponseWriter, r *http.Request, params types.ListStagesParams) {
	a.S.ServeHTTP(w, r)
}

// CreateStage delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) CreateStage(w http.ResponseWriter, r *http.Request, params types.CreateStageParams) {
	a.S.ServeHTTP(w, r)
}

// GetStage delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) GetStage(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.S.ServeHTTP(w, r)
}

// UpdateStage delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) UpdateStage(w http.ResponseWriter, r *http.Request, idParam types.IdParam, params types.UpdateStageParams) {
	a.S.ServeHTTP(w, r)
}

// ArchiveStage delegates to the wired handler; see the struct doc comment above.
func (a *PipelinesAdapter) ArchiveStage(w http.ResponseWriter, r *http.Request, idParam types.IdParam) {
	a.S.ServeHTTP(w, r)
}
