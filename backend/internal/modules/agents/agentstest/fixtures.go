// Package agentstest holds integration-test-only fixtures shared across the
// agents module's `_test` packages (agents/app and agents/adapters). Go's
// `_test.go` files cannot import another package's test-only symbols, and
// agents/app and agents/adapters are two different Go packages
// (`app_test`/`adapters_test`), so a plain (non-`_test.go`) file is the only
// way to share a fixture between them without duplicating it — mirroring the
// project's existing internal/shared/kernel/pgtest precedent (a normal
// package, like net/http/httptest, that takes *testing.T and is only ever
// imported from test files).
//
// Only the pipeline/stage seeding fixture that both packages need identically
// lives here; anything module-specific stays local to its own test file.
package agentstest

import (
	"context"
	"database/sql"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// StageSpec describes one pipeline stage to seed via SeedPipeline.
type StageSpec struct {
	Name     string
	Position int
	Semantic string
	WinProb  int
}

// SeedPipeline inserts a pipeline and its stages (in the given order) for
// workspace wsID, returning the new pipeline id and the seeded stage ids (in
// the same order as stages).
func SeedPipeline(t *testing.T, db *sql.DB, wsID string, stages []StageSpec) (string, []struct{ ID string }) {
	t.Helper()
	var pipelineID string
	if err := db.QueryRowContext(context.Background(), `INSERT INTO pipeline (workspace_id, name) VALUES ($1::uuid,$2) RETURNING id`, wsID, "agents-pipeline-"+ids.New()).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	out := make([]struct{ ID string }, 0, len(stages))
	for _, s := range stages {
		var id string
		if err := db.QueryRowContext(context.Background(), `INSERT INTO stage (workspace_id, pipeline_id, name, position, semantic, win_probability)
			VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6) RETURNING id`, wsID, pipelineID, s.Name, s.Position, s.Semantic, s.WinProb).Scan(&id); err != nil {
			t.Fatalf("seed stage %q: %v", s.Name, err)
		}
		out = append(out, struct{ ID string }{ID: id})
	}
	return pipelineID, out
}
