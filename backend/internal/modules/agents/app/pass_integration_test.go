//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestRunPass_NoisyFixtureEndToEndThroughTheRealAssembler(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	view := domain.AssembledView{WorkspaceID: wsID, WindowStart: time.Now().Add(-24 * time.Hour), WindowEnd: time.Now()}
	conf9, conf5, conf0 := 0.9, 0.5, 0.0
	produce := func(domain.AssembledView) ([]domain.Proposal, error) {
		return []domain.Proposal{
			{WorkspaceID: wsID, ActionType: "log_link", TargetEntity: "activity:1", Evidence: "e1", Confidence: &conf9, Source: "s1", Effect: json.RawMessage(`{}`)},
			{WorkspaceID: wsID, ActionType: "send", TargetEntity: "deal:1", Evidence: "e2", Confidence: &conf5, Source: "s2", Effect: json.RawMessage(`{}`)},
			{WorkspaceID: wsID, ActionType: "send", TargetEntity: "deal:2", Evidence: "", Confidence: &conf0, Source: "s3", Effect: json.RawMessage(`{}`)}, // no-guess drop: empty evidence
		}, nil
	}
	effector := &spyEffector{rollbackHandle: "rb-1"}
	emitter := &spyEmitter{}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view}, // the seam is genuinely on the pass's critical path (OVN-EVT-1)
		Since:       view.WindowStart,
		Produce:     produce,
		Stage:       crmapprovals.Stage, // the real stager — never a second staging mechanism
		Repo:        repo,
		Effector:    effector,
		Emitter:     emitter,
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
	// 2 groups survive the gate: "log_link" (🟢 applied) and "send" (🟡
	// staged) — the second "send" fixture is dropped for empty evidence.
	if len(result.Groups) != 2 {
		t.Fatalf("groups = %+v, want 2 (log_link, send)", result.Groups)
	}
	if !effector.called {
		t.Fatal("expected the \"log_link\" proposal to be applied 🟢")
	}

	pending, err := repo.ListByStatus(context.Background(), db, wsID, crmapprovals.StatusPending)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 || pending[0].ActionType != "overnight.send" {
		t.Fatalf("expected exactly one overnight.send item staged pending, got %+v", pending)
	}
}

func TestRunPass_AssemblerErrorDegradesTheRun(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	assemblerErr := errors.New("assembler: upstream capture stream unavailable")

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{Err: assemblerErr},
		Since:       time.Now(),
		Produce: func(domain.AssembledView) ([]domain.Proposal, error) {
			t.Fatal("Produce must not run when the assembler itself failed")
			return nil, nil
		},
	})
	_ = tx.Rollback()
	if err != nil {
		t.Fatalf("RunPass must return the degraded RunResult, not an error: %v", err)
	}
	if result.State != domain.RunDegraded {
		t.Fatalf("state = %v, want RunDegraded", result.State)
	}
	if result.DegradedReason != assemblerErr.Error() {
		t.Fatalf("DegradedReason = %q, want %q", result.DegradedReason, assemblerErr.Error())
	}
}
