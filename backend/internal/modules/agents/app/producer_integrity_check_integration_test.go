//go:build integration

package app_test

import (
	"context"
	"testing"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// integritySupportedFacts is one fully-supported record per check -
// deal:1 (call), deal:4 (mail), deal:7 (meeting), deal:10 (stage) - used
// to prove the single most important negative case: a record the
// evidence supports produces zero flags. Ticket-scoped name (not
// "supportedFacts") - this file shares package app_test with ONA-T03's
// own *_integration_test.go files on a sibling branch; a generic helper
// name risks a build-breaking collision when both land.
func integritySupportedFacts() []domain.Fact {
	return []domain.Fact{
		{EntityType: "call_claim", EntityID: "deal:1", Detail: `{"confidence":0.9,"description":"call logged re: renewal"}`, Source: "capture:call:1"},
		{EntityType: "call_trace", EntityID: "deal:1", Source: "capture:calendar:1"},

		{EntityType: "proposal_sent_claim", EntityID: "deal:4", Detail: `{"confidence":0.85,"description":"proposal sent to buyer"}`, Source: "capture:stage:4"},
		{EntityType: "outbound_email_trace", EntityID: "deal:4", Source: "capture:email:4"},

		{EntityType: "meeting_claim", EntityID: "deal:7", Detail: `{"confidence":0.8,"description":"discovery meeting logged"}`, Source: "capture:meeting:7"},
		{EntityType: "meeting_recap_trace", EntityID: "deal:7", Source: "capture:recap:7"},

		{EntityType: "stage_claim", EntityID: "deal:10", Detail: `{"confidence":0.9,"stage":"negotiation"}`, Source: "capture:stage:10"},
		{EntityType: "stage_signal", EntityID: "deal:10", Detail: `{"confidence":0.88,"supports_stage":"negotiation"}`, Source: "capture:signal:10"},
	}
}

// integrityMixedFacts adds one genuinely-contradicting record per check
// (deal:2, deal:5, deal:8, deal:11 - deal:11 also carries a stage_signal
// so its contradiction also emits a stage_correction) and one malformed/
// missing-confidence claim per check (deal:3, deal:6, deal:9, deal:12),
// on top of the supported set. Ticket-scoped name, same reason as
// integritySupportedFacts above.
func integrityMixedFacts() []domain.Fact {
	out := append([]domain.Fact{}, integritySupportedFacts()...)
	out = append(
		out,
		domain.Fact{EntityType: "call_claim", EntityID: "deal:2", Detail: `{"confidence":0.75,"description":"call logged re: pricing"}`, Source: "capture:call:2"},
		domain.Fact{EntityType: "call_claim", EntityID: "deal:3", Detail: `{"description":"call logged, no confidence"}`, Source: "capture:call:3"},

		domain.Fact{EntityType: "proposal_sent_claim", EntityID: "deal:5", Detail: `{"confidence":0.6,"description":"proposal sent to buyer"}`, Source: "capture:stage:5"},
		domain.Fact{EntityType: "proposal_sent_claim", EntityID: "deal:6", Detail: `{"confidence":0.5}`, Source: "capture:stage:6"},

		domain.Fact{EntityType: "meeting_claim", EntityID: "deal:8", Detail: `{"confidence":0.65,"description":"discovery meeting logged"}`, Source: "capture:meeting:8"},
		domain.Fact{EntityType: "meeting_claim", EntityID: "deal:9", Detail: `not-json`, Source: "capture:meeting:9"},

		domain.Fact{EntityType: "stage_claim", EntityID: "deal:11", Detail: `{"confidence":0.7,"stage":"proposal_sent"}`, Source: "capture:stage:11"},
		domain.Fact{EntityType: "stage_signal", EntityID: "deal:11", Detail: `{"confidence":0.82,"supports_stage":"negotiation"}`, Source: "capture:signal:11"},
		domain.Fact{EntityType: "stage_claim", EntityID: "deal:12", Detail: `{"stage":"negotiation"}`, Source: "capture:stage:12"},
	)
	return out
}

func TestIntegrityCheckProduce_SupportedFixtureAloneYieldsRunQuiet(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	view := domain.AssembledView{WorkspaceID: wsID, Facts: integritySupportedFacts()}
	repo := crmapprovals.NewRepository()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view},
		Since:       view.WindowStart,
		Produce:     app.IntegrityCheckProduce,
		Stage:       crmapprovals.Stage,
		Repo:        repo,
		Effector:    &spyEffector{},
		Emitter:     &spyEmitter{},
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("RunPass: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if result.State != domain.RunQuiet {
		t.Fatalf("state = %v, want RunQuiet (a fully-supported record must yield zero flags)", result.State)
	}
}

func TestIntegrityCheckProduce_MixedFixtureFlagsContradictionsNeverAutoApplies(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	view := domain.AssembledView{WorkspaceID: wsID, Facts: integrityMixedFacts()}
	repo := crmapprovals.NewRepository()
	effector := &spyEffector{rollbackHandle: "rb-should-never-be-used"}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view},
		Since:       view.WindowStart,
		Produce:     app.IntegrityCheckProduce,
		Stage:       crmapprovals.Stage,
		Repo:        repo,
		Effector:    effector,
		Emitter:     &spyEmitter{},
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("RunPass: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if result.State != domain.RunNormal {
		t.Fatalf("state = %v, want RunNormal", result.State)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("groups = %+v, want 2 (integrity_flag, stage_correction)", result.Groups)
	}
	var sawFlag, sawCorrection, total int
	for _, g := range result.Groups {
		for _, item := range g.Items {
			if item.Evidence == "" || item.Confidence == nil {
				t.Fatalf("item %+v missing evidence/confidence", item)
			}
			total++
		}
		switch g.ActionType {
		case "integrity_flag":
			sawFlag = len(g.Items)
		case "stage_correction":
			sawCorrection = len(g.Items)
		default:
			t.Fatalf("unexpected group action type %q", g.ActionType)
		}
	}
	if sawFlag != 4 {
		t.Fatalf("integrity_flag group size = %d, want 4 (call, mail, meeting, stage)", sawFlag)
	}
	if sawCorrection != 1 {
		t.Fatalf("stage_correction group size = %d, want 1", sawCorrection)
	}
	if total != 5 {
		t.Fatalf("total surviving proposals = %d, want 5", total)
	}
	if effector.called {
		t.Fatal("Effector.Apply must never be called for integrity_flag or stage_correction - 🟡 only, never automatic")
	}

	pending, err := repo.ListByStatus(context.Background(), db, wsID, crmapprovals.StatusPending)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 5 {
		t.Fatalf("expected 5 staged pending items, got %d", len(pending))
	}
}
