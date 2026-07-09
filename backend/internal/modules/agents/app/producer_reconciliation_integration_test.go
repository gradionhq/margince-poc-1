//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

// TestRunPass_ReconciliationProduceEndToEnd proves OVN-AC-1/OVN-AC-3 for
// this ticket's producer: a noisy fixture spanning all three signal types
// (each with one valid + one deliberately-malformed fact) drives RunPass
// (app/pass.go, untouched by this ticket) exactly as a future runner
// will call it, using ReconciliationProduce as PassInput.Produce.
func TestRunPass_ReconciliationProduceEndToEnd(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	validFieldChange := mustJSON(map[string]any{
		"deal_id": "deal-1", "field": "stage", "value": json.RawMessage(`"negotiation"`),
		"evidence": "call transcript", "confidence": 0.9,
	})
	malformedFieldChange := mustJSON(map[string]any{
		"deal_id": "deal-1", "field": "close_date", "value": json.RawMessage(`"2026-08-01"`), // ONA-T02's field, never this producer's
		"evidence": "e", "confidence": 0.5,
	})
	validNewContact := mustJSON(map[string]any{
		"name": "Jane Roe", "email": "jane@example.com", "relates_to": "deal:deal-1",
		"evidence": "email signature block", "confidence": 0.85,
	})
	malformedNewContact := mustJSON(map[string]any{
		"name": "No Contact Info", "relates_to": "deal:deal-1", // no email AND no phone
		"evidence": "e", "confidence": 0.5,
	})
	validFollowUp := mustJSON(map[string]any{
		"target": "deal:deal-1", "recipient": "jane@example.com", "subject": "Following up",
		"body": "Great chatting today...", "evidence": "call recap", "confidence": 0.8,
	})
	malformedFollowUp := mustJSON(map[string]any{
		"target": "deal:deal-1", "body": "", // empty draft -- never invented
		"evidence": "e", "confidence": 0.5,
	})

	view := domain.AssembledView{
		WorkspaceID: wsID,
		WindowStart: time.Now().Add(-24 * time.Hour),
		WindowEnd:   time.Now(),
		Facts: []domain.Fact{
			{EntityType: app.EntityTypeFieldChangeSignal, EntityID: "f1", Detail: validFieldChange, Source: "capture:call:1"},
			{EntityType: app.EntityTypeFieldChangeSignal, EntityID: "f2", Detail: malformedFieldChange, Source: "capture:call:2"},
			{EntityType: app.EntityTypeNewContactSignal, EntityID: "f3", Detail: validNewContact, Source: "capture:email:1"},
			{EntityType: app.EntityTypeNewContactSignal, EntityID: "f4", Detail: malformedNewContact, Source: "capture:email:2"},
			{EntityType: app.EntityTypeFollowUpSignal, EntityID: "f5", Detail: validFollowUp, Source: "capture:call:3"},
			{EntityType: app.EntityTypeFollowUpSignal, EntityID: "f6", Detail: malformedFollowUp, Source: "capture:call:4"},
		},
	}

	effector := &spyEffector{rollbackHandle: "rb-1"}
	emitter := &spyEmitter{}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	result, err := app.RunPass(context.Background(), tx, app.PassInput{
		WorkspaceID: wsID,
		Assembler:   ports.FixtureAssembler{View: view},
		Since:       view.WindowStart,
		Produce:     app.ReconciliationProduce,
		Stage:       crmapprovals.Stage,
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
	// Exactly 3 groups: the one deliberately-malformed fact per type is
	// dropped by the gate each producer holds itself (OVN-AC-1).
	if len(result.Groups) != 3 {
		t.Fatalf("groups = %+v, want 3 (create_contact, draft_followup, field_change)", result.Groups)
	}
	wantTypes := map[string]bool{"field_change": true, "create_contact": true, "draft_followup": true}
	for _, g := range result.Groups {
		if !wantTypes[g.ActionType] {
			t.Fatalf("unexpected group ActionType %q", g.ActionType)
		}
		if len(g.Items) != 1 {
			t.Fatalf("group %q: got %d items, want 1 (the malformed fact must not survive)", g.ActionType, len(g.Items))
		}
		item := g.Items[0]
		if item.Evidence == "" || item.Confidence == nil {
			t.Fatalf("group %q: item missing evidence/confidence (OVN-AC-1): %+v", g.ActionType, item)
		}
	}

	// OVN-AC-1: staged yellow-tier, unsent -- commit nothing outward for
	// any of the three action types.
	if effector.called {
		t.Fatal("Effector.Apply must never be called -- none of the three action types is ever 🟢")
	}
	pending, err := repo.ListByStatus(context.Background(), db, wsID, crmapprovals.StatusPending)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected exactly 3 pending approval items (one per action type), got %d", len(pending))
	}
	wantNamespaced := map[string]bool{"overnight.field_change": true, "overnight.create_contact": true, "overnight.draft_followup": true}
	for _, item := range pending {
		if !wantNamespaced[item.ActionType] {
			t.Fatalf("unexpected staged ActionType %q", item.ActionType)
		}
	}
}
