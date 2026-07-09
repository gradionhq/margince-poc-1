//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" driver for database/sql.Open

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

func TestRunPass_StalledRecoveryProduceStagesOnlySupportedDeal(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	view := domain.AssembledView{
		WorkspaceID: wsID,
		WindowStart: time.Now().Add(-24 * time.Hour),
		WindowEnd:   time.Now(),
		Facts: []domain.Fact{
			{EntityType: "deal_stalled_claim", EntityID: "1", Detail: `{"generic_reason":"no_activity_60_days","wait_until_active":true,"confidence":0.9}`, Source: "capture:stalled:1"},
			{EntityType: "recovery_evidence_signal", EntityID: "1", Detail: `{"specific_reason":"no_reply_14_days","evidence_activity_id":"act-1","evidence_text":"no reply since last email","confidence":0.8}`, Source: "capture:evidence:1"},
			{EntityType: "deal_stalled_claim", EntityID: "2", Detail: `{"generic_reason":"no_activity_60_days","confidence":0.85}`, Source: "capture:stalled:2"},
			{EntityType: "recovery_evidence_signal", EntityID: "2", Detail: `{"specific_reason":"champion_quiet","evidence_activity_id":"act-2","evidence_text":"champion has been quiet","confidence":0.7}`, Source: "capture:evidence:2"},
			{EntityType: "recovery_draft_signal", EntityID: "2", Detail: `{"subject":"Checking in","body":"Hi, just checking in on this.","confidence":0.6}`, Source: "capture:draft:2"},
		},
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view},
		Since:       view.WindowStart,
		Produce:     app.StalledRecoveryProduce,
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

	if result.State != domain.RunNormal {
		t.Fatalf("state = %v, want RunNormal", result.State)
	}
	pending, err := repo.ListByStatus(context.Background(), db, wsID, crmapprovals.StatusPending)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected exactly 1 staged pending item, got %d", len(pending))
	}
	item := pending[0]
	if item.ActionType != "overnight.stalled_recovery" {
		t.Fatalf("ActionType = %q, want overnight.stalled_recovery", item.ActionType)
	}
	if item.RequestedBy != app.ActorOvernight {
		t.Fatalf("RequestedBy = %q, want %q", item.RequestedBy, app.ActorOvernight)
	}
	var effect struct {
		Reason             string `json:"reason"`
		EvidenceActivityID string `json:"evidence_activity_id"`
		DealID             string `json:"deal_id"`
		WorkspaceID        string `json:"workspace_id"`
		Draft              *struct {
			Subject string `json:"subject"`
			Body    string `json:"body"`
		} `json:"draft"`
	}
	if err := json.Unmarshal(item.Payload, &effect); err != nil {
		t.Fatalf("unmarshal staged payload: %v", err)
	}
	if effect.DealID != "2" || effect.WorkspaceID != wsID || effect.Reason != "champion_quiet" {
		t.Fatalf("staged payload mismatch: %+v", effect)
	}
	if effect.Draft == nil || effect.Draft.Subject != "Checking in" {
		t.Fatalf("staged draft mismatch: %+v", effect.Draft)
	}
}

func TestStalledRecoveryProduce_SuppressedFixtureAloneYieldsRunQuiet(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	view := domain.AssembledView{WorkspaceID: wsID, Facts: []domain.Fact{
		{EntityType: "deal_stalled_claim", EntityID: "1", Detail: `{"generic_reason":"no_activity_60_days","wait_until_active":true,"confidence":0.9}`, Source: "capture:stalled:1"},
		{EntityType: "recovery_evidence_signal", EntityID: "1", Detail: `{"specific_reason":"no_reply_14_days","evidence_activity_id":"act-1","evidence_text":"no reply","confidence":0.8}`, Source: "capture:evidence:1"},
	}}
	repo := crmapprovals.NewRepository()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view},
		Since:       view.WindowStart,
		Produce:     app.StalledRecoveryProduce,
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
		t.Fatalf("state = %v, want RunQuiet (an asked-to-wait deal must never be falsely flagged, OVN-AC-6)", result.State)
	}
}

func TestStalledRecoveryProduce_StalledWithNoDraftFixtureStillStagesWithNullDraft(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	view := domain.AssembledView{WorkspaceID: wsID, Facts: []domain.Fact{
		{EntityType: "deal_stalled_claim", EntityID: "3", Detail: `{"generic_reason":"no_activity_60_days","confidence":0.75}`, Source: "capture:stalled:3"},
		{EntityType: "recovery_evidence_signal", EntityID: "3", Detail: `{"specific_reason":"missed_follow_up","evidence_activity_id":"act-3","evidence_text":"promised follow-up never sent","confidence":0.65}`, Source: "capture:evidence:3"},
	}}
	repo := crmapprovals.NewRepository()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view},
		Since:       view.WindowStart,
		Produce:     app.StalledRecoveryProduce,
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
	if result.State != domain.RunNormal {
		t.Fatalf("state = %v, want RunNormal", result.State)
	}
	pending, err := repo.ListByStatus(context.Background(), db, wsID, crmapprovals.StatusPending)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected exactly one staged item, got %d", len(pending))
	}
	var effect struct {
		Draft *struct{} `json:"draft"`
	}
	if err := json.Unmarshal(pending[0].Payload, &effect); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if effect.Draft != nil {
		t.Fatalf("staged payload draft = %+v, want null (never fabricated, OVN-AC-5 draft degradation)", effect.Draft)
	}
}
