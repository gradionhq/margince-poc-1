//go:build integration

package deals

import (
	"context"
	"testing"
)

// wsDevSeed is the fixed workspace id backend/seed/dev.sql seeds into.
const wsDevSeed = "00000000-0000-0000-0000-000000000001"

// TestDevSeed_DefaultPipeline_HasSevenPinnedStages asserts DEAL-AC-13/
// DEAL-FORM-1: exactly one default pipeline in the dev seed workspace, with
// its seven pinned stages at the pinned positions/probabilities. This test
// only passes against a DB that has run `make seed-reset` (or the migrate +
// seed step of `make test-integration`'s bootstrap) with the current
// backend/seed/dev.sql.
func TestDevSeed_DefaultPipeline_HasSevenPinnedStages(t *testing.T) {
	db := openTestDB(t)
	setRLS(t, db, wsDevSeed)
	ctx := context.Background()

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
