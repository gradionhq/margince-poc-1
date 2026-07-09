package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func stalledRecoveryFact(entityID, detail, source string) domain.Fact {
	return domain.Fact{EntityType: "deal_stalled_claim", EntityID: entityID, Detail: detail, Source: source}
}

func evidenceSignalFact(entityID, detail, source string) domain.Fact {
	return domain.Fact{EntityType: "recovery_evidence_signal", EntityID: entityID, Detail: detail, Source: source}
}

func draftSignalFact(entityID, detail, source string) domain.Fact {
	return domain.Fact{EntityType: "recovery_draft_signal", EntityID: entityID, Detail: detail, Source: source}
}

func TestStalledRecoveryProduce_SuppressedByActiveWaitYieldsZeroProposals(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		stalledRecoveryFact("1", `{"generic_reason":"no_activity_60_days","wait_until_active":true,"confidence":0.9}`, "capture:stalled:1"),
		evidenceSignalFact("1", `{"specific_reason":"no_reply_14_days","evidence_activity_id":"act-1","evidence_text":"no reply since last email","confidence":0.8}`, "capture:evidence:1"),
	}}
	out, err := app.StalledRecoveryProduce(view)
	if err != nil {
		t.Fatalf("StalledRecoveryProduce: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 proposals for a wait_until_active suppression, got %d: %+v", len(out), out)
	}
}

func TestStalledRecoveryProduce_StalledWithEvidenceAndDraftYieldsOneFullProposal(t *testing.T) {
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		stalledRecoveryFact("2", `{"generic_reason":"no_activity_60_days","wait_until_active":false,"confidence":0.85}`, "capture:stalled:2"),
		evidenceSignalFact("2", `{"specific_reason":"no_reply_14_days","evidence_activity_id":"act-2","evidence_text":"no reply in 14 days","confidence":0.7}`, "capture:evidence:2"),
		draftSignalFact("2", `{"subject":"Checking in","body":"Hi, just checking in on this.","confidence":0.6}`, "capture:draft:2"),
	}}
	out, err := app.StalledRecoveryProduce(view)
	if err != nil {
		t.Fatalf("StalledRecoveryProduce: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 proposal, got %d: %+v", len(out), out)
	}
	p := out[0]
	if p.ActionType != "stalled_recovery" {
		t.Errorf("ActionType = %q, want stalled_recovery", p.ActionType)
	}
	if p.TargetEntity != "deal:2" {
		t.Errorf("TargetEntity = %q, want deal:2", p.TargetEntity)
	}
	if p.Evidence == "" {
		t.Error("Evidence must be non-empty")
	}
	if p.Confidence == nil || *p.Confidence != 0.7 {
		t.Errorf("Confidence = %v, want 0.7 (the evidence signal's, never the draft's)", p.Confidence)
	}
	if p.Source != "capture:evidence:2" {
		t.Errorf("Source = %q, want capture:evidence:2 (the evidence signal's Source)", p.Source)
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
	if err := json.Unmarshal(p.Effect, &effect); err != nil {
		t.Fatalf("Effect unmarshal: %v", err)
	}
	if effect.Reason != "no_reply_14_days" {
		t.Errorf("Effect.reason = %q, want no_reply_14_days", effect.Reason)
	}
	if effect.EvidenceActivityID != "act-2" {
		t.Errorf("Effect.evidence_activity_id = %q, want act-2", effect.EvidenceActivityID)
	}
	if effect.DealID != "2" {
		t.Errorf("Effect.deal_id = %q, want 2", effect.DealID)
	}
	if effect.WorkspaceID != "ws-1" {
		t.Errorf("Effect.workspace_id = %q, want ws-1", effect.WorkspaceID)
	}
	if effect.Draft == nil || effect.Draft.Subject != "Checking in" || effect.Draft.Body != "Hi, just checking in on this." {
		t.Errorf("Effect.draft = %+v, want the drafted subject/body", effect.Draft)
	}
}

func TestStalledRecoveryProduce_StalledWithEvidenceNoDraftYieldsNullDraft(t *testing.T) {
	view := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{
		stalledRecoveryFact("3", `{"generic_reason":"no_activity_60_days","confidence":0.75}`, "capture:stalled:3"),
		evidenceSignalFact("3", `{"specific_reason":"missed_follow_up","evidence_activity_id":"act-3","evidence_text":"promised follow-up never sent","confidence":0.65}`, "capture:evidence:3"),
	}}
	out, err := app.StalledRecoveryProduce(view)
	if err != nil {
		t.Fatalf("StalledRecoveryProduce: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(out))
	}
	var effect struct {
		Draft *struct{} `json:"draft"`
	}
	if err := json.Unmarshal(out[0].Effect, &effect); err != nil {
		t.Fatalf("Effect unmarshal: %v", err)
	}
	if effect.Draft != nil {
		t.Errorf("Effect.draft = %+v, want null (never fabricated)", effect.Draft)
	}
}

func TestStalledRecoveryProduce_StalledWithNoEvidenceSignalYieldsZeroProposals(t *testing.T) {
	view := domain.AssembledView{Facts: []domain.Fact{
		stalledRecoveryFact("4", `{"generic_reason":"no_activity_60_days","confidence":0.9}`, "capture:stalled:4"),
	}}
	out, err := app.StalledRecoveryProduce(view)
	if err != nil {
		t.Fatalf("StalledRecoveryProduce: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 proposals (no-guess: no evidence, no flag), got %d: %+v", len(out), out)
	}
}

func TestStalledRecoveryProduce_MalformedClaimOrEvidenceYieldsZeroProposals(t *testing.T) {
	cases := map[string]domain.AssembledView{
		"claim not json": {Facts: []domain.Fact{
			stalledRecoveryFact("5", `not-json`, "capture:stalled:5"),
			evidenceSignalFact("5", `{"specific_reason":"no_reply_14_days","evidence_activity_id":"act-5","evidence_text":"t","confidence":0.5}`, "capture:evidence:5"),
		}},
		"claim missing confidence": {Facts: []domain.Fact{
			stalledRecoveryFact("6", `{"generic_reason":"no_activity_60_days","wait_until_active":false}`, "capture:stalled:6"),
			evidenceSignalFact("6", `{"specific_reason":"no_reply_14_days","evidence_activity_id":"act-6","evidence_text":"t","confidence":0.5}`, "capture:evidence:6"),
		}},
		"evidence not json": {Facts: []domain.Fact{
			stalledRecoveryFact("7", `{"generic_reason":"no_activity_60_days","confidence":0.9}`, "capture:stalled:7"),
			evidenceSignalFact("7", `not-json`, "capture:evidence:7"),
		}},
		"evidence missing confidence": {Facts: []domain.Fact{
			stalledRecoveryFact("8", `{"generic_reason":"no_activity_60_days","confidence":0.9}`, "capture:stalled:8"),
			evidenceSignalFact("8", `{"specific_reason":"champion_quiet","evidence_activity_id":"act-8","evidence_text":"t"}`, "capture:evidence:8"),
		}},
	}
	for name, view := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := app.StalledRecoveryProduce(view)
			if err != nil {
				t.Fatalf("StalledRecoveryProduce: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("%s: expected 0 proposals, got %d", name, len(out))
			}
		})
	}
}
