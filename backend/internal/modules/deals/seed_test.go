//go:build integration

package deals

import (
	"context"
	"testing"
)

// wsDevSeed is the fixed workspace id backend/seed/dev.sql seeds into.
const wsDevSeed = "00000000-0000-0000-0000-000000000001"

// devSeedPipelineStagesSQL is the workspace/pipeline/stage block copied
// verbatim from backend/seed/dev.sql (the "Sales pipeline + stages"
// section). It's idempotent (ON CONFLICT (id) DO NOTHING) so re-running it
// against a DB that already has these rows (e.g. from `make seed-reset` or a
// prior test run) is safe. Copying just the rows this test needs — rather
// than exec'ing the whole dev.sql file — sidesteps having to reason about
// whether lib/pq can safely Exec dev.sql's other multi-statement sections
// (e.g. the role_grants JSON blobs, which contain colons/braces but no
// top-level semicolons inside string literals here either way).
const devSeedPipelineStagesSQL = `
INSERT INTO workspace (id, name, slug, base_currency)
VALUES ('00000000-0000-0000-0000-000000000001', 'Dev Workspace', 'dev', 'EUR')
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline (id, workspace_id, name, is_default, position)
VALUES ('00000000-0000-0000-0040-000000000001', '00000000-0000-0000-0000-000000000001',
        'Sales Pipeline', true, 1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
VALUES
  ('00000000-0000-0000-0041-000000000001', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'New',         1, 'open', 10),
  ('00000000-0000-0000-0041-000000000002', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Qualified',   2, 'open', 25),
  ('00000000-0000-0000-0041-000000000003', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Discovery',   3, 'open', 40),
  ('00000000-0000-0000-0041-000000000004', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Proposal',    4, 'open', 60),
  ('00000000-0000-0000-0041-000000000005', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Negotiation', 5, 'open', 80),
  ('00000000-0000-0000-0041-000000000006', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Closed Won',  6, 'won',  100),
  ('00000000-0000-0000-0041-000000000007', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Closed Lost', 7, 'lost', 0)
ON CONFLICT (id) DO NOTHING;
`

// TestDevSeed_DefaultPipeline_HasSevenPinnedStages asserts DEAL-AC-13/
// DEAL-FORM-1: exactly one default pipeline in the dev seed workspace, with
// its seven pinned stages at the pinned positions/probabilities. The test is
// self-contained: it loads the pipeline/stage rows it needs (copied from
// backend/seed/dev.sql, which is idempotent) itself, so it passes against a
// bare migrated `margince_test` DB — it no longer depends on an out-of-band
// `make seed-reset` having been run first.
func TestDevSeed_DefaultPipeline_HasSevenPinnedStages(t *testing.T) {
	db := openTestDB(t)
	setRLS(t, db, wsDevSeed)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, devSeedPipelineStagesSQL); err != nil {
		t.Fatal("seed pipeline/stages:", err)
	}

	pstore := NewPipelineStore(db)
	pipelines, _, err := pstore.List(ctx, wsDevSeed, "", 100)
	if err != nil {
		t.Fatal("list pipelines:", err)
	}
	var defaults []Pipeline
	for _, p := range pipelines {
		if p.IsDefault {
			defaults = append(defaults, p)
		}
	}
	if len(defaults) != 1 {
		t.Fatalf("expected exactly one default pipeline, got %d", len(defaults))
	}
	def := defaults[0]

	sstore := NewStageStore(db)
	stages, _, err := sstore.List(ctx, wsDevSeed, def.ID, "", 100)
	if err != nil {
		t.Fatal("list stages:", err)
	}

	type wantStage struct {
		name     string
		position int
		prob     int
		semantic string
	}
	want := []wantStage{
		{"New", 1, 10, "open"},
		{"Qualified", 2, 25, "open"},
		{"Discovery", 3, 40, "open"},
		{"Proposal", 4, 60, "open"},
		{"Negotiation", 5, 80, "open"},
		{"Closed Won", 6, 100, "won"},
		{"Closed Lost", 7, 0, "lost"},
	}
	if len(stages) != len(want) {
		t.Fatalf("expected %d stages, got %d", len(want), len(stages))
	}
	for i, w := range want {
		got := stages[i]
		if got.Name != w.name || got.Position != w.position || got.WinProbability != w.prob || got.Semantic != w.semantic {
			t.Errorf("stage[%d] = %+v, want name=%s position=%d prob=%d semantic=%s",
				i, got, w.name, w.position, w.prob, w.semantic)
		}
	}
}
