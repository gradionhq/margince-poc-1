// Package ports defines the storage interfaces for the deals module.
package ports

import (
	"context"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
)

// PipelineStorer is the persistence seam for pipeline rows.
type PipelineStorer interface {
	Create(ctx context.Context, pl domain.Pipeline) (domain.Pipeline, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Pipeline, error)
	List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Pipeline, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any) (domain.Pipeline, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Pipeline, error)
}

// StageStorer is the persistence seam for stage rows.
type StageStorer interface {
	Create(ctx context.Context, st domain.Stage) (domain.Stage, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Stage, error)
	List(ctx context.Context, workspaceID, pipelineID, cursor string, limit int) ([]domain.Stage, string, error)
	Update(ctx context.Context, id, workspaceID string, updates map[string]any) (domain.Stage, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Stage, error)
}

// RollupStorer is the persistence seam for pipeline roll-up reads.
type RollupStorer interface {
	Get(ctx context.Context, pipelineID, workspaceID string, asOf time.Time) (domain.PipelineRollup, error)
}
