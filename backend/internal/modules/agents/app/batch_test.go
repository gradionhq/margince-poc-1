package app_test

import (
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func routed(actionType string, conf float64) domain.RoutedProposal {
	c := conf
	return app.RouteTier(domain.Proposal{ActionType: actionType, Confidence: &c})
}

func TestBuildBatch_QuietFixtureYieldsHonestEmpty(t *testing.T) {
	result := app.BuildBatch(nil, nil)
	if result.State != domain.RunQuiet {
		t.Fatalf("state = %v, want RunQuiet", result.State)
	}
	if len(result.Groups) != 0 {
		t.Fatalf("expected zero groups on a quiet run, got %d", len(result.Groups))
	}
}

func TestBuildBatch_DegradedFixtureCarriesReason(t *testing.T) {
	producerErr := errors.New("assembler timed out mid-window")
	partial := []domain.RoutedProposal{routed("log_link", 0.5)}
	result := app.BuildBatch(partial, producerErr)
	if result.State != domain.RunDegraded {
		t.Fatalf("state = %v, want RunDegraded", result.State)
	}
	if result.DegradedReason != producerErr.Error() {
		t.Fatalf("DegradedReason = %q, want %q", result.DegradedReason, producerErr.Error())
	}
	if len(result.Groups) == 0 {
		t.Fatal("a degraded run still returns whatever survived — expected 1 group")
	}
}

func TestBuildBatch_NoisyFixtureIsGroupedAndRanked(t *testing.T) {
	in := []domain.RoutedProposal{
		routed("log_link", 0.4),
		routed("log_link", 0.9),
		routed("close-deal", 0.6),
		routed("log_link", 0.7),
	}
	result := app.BuildBatch(in, nil)
	if result.State != domain.RunNormal {
		t.Fatalf("state = %v, want RunNormal", result.State)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("expected 2 groups (log_link, close-deal), got %d", len(result.Groups))
	}
	for _, g := range result.Groups {
		if g.ActionType != "log_link" {
			continue
		}
		if len(g.Items) != 3 {
			t.Fatalf("log_link group size = %d, want 3", len(g.Items))
		}
		for i := 1; i < len(g.Items); i++ {
			if *g.Items[i-1].Confidence < *g.Items[i].Confidence {
				t.Fatalf("log_link group not ranked descending by confidence: %+v", g.Items)
			}
		}
	}
}
