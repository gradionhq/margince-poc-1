package app

import (
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
)

// Correlated by shared `EntityID` = the deal's raw ID, unprefixed (e.g. "1",
// not "deal:1") — this differs from ONA-T03/ONA-T04's own fixture convention
// (`EntityID` already `"deal:<id>"`, `TargetEntity` copied verbatim) because
// this spec's Task 1 explicitly states `TargetEntity = "deal:<id>"` as a
// derived value, distinct from the raw `EntityID` the three Fact types
// correlate on:
//
// | `EntityType` | `Detail` JSON | Notes |
// |---|---|---|
// | `deal_stalled_claim` | `{"generic_reason": "no_activity_60_days", "wait_until_active": bool, "confidence": <float>}` | `generic_reason`+`confidence` required — malformed/missing-confidence skipped entirely. `wait_until_active: true` → **no flag at all** (OVN-AC-6) — the other two Fact types for that `EntityID` are never even inspected. |
// | `recovery_evidence_signal` (same `EntityID`) | `{"specific_reason": "no_reply_14_days"\|"missed_follow_up"\|"champion_quiet", "evidence_activity_id": "...", "evidence_text": "...", "confidence": <float>}` | **Required** for a flag to exist — a stalled claim with no evidence signal produces nothing (no-guess: never falls back to the generic reason as if it were specific). Multiple signals for the same deal → highest-confidence one wins. |
// | `recovery_draft_signal` (same `EntityID`, optional) | `{"subject": "...", "body": "...", "confidence": <float>}` | Absent (or malformed) → `Effect.draft` is `null` — never fabricated (OVN-AC-5 draft degradation). Multiple drafts → highest-confidence one wins, same convention as the evidence signal. |
type stalledRecoveryEffect struct {
	Reason             string            `json:"reason"`
	EvidenceActivityID string            `json:"evidence_activity_id"`
	DealID             string            `json:"deal_id"`
	WorkspaceID        string            `json:"workspace_id"`
	Draft              map[string]string `json:"draft"`
}

type stalledClaim struct {
	GenericReason   string   `json:"generic_reason"`
	WaitUntilActive bool     `json:"wait_until_active"`
	Confidence      *float64 `json:"confidence"`
}

type stalledEvidenceSignal struct {
	SpecificReason     string   `json:"specific_reason"`
	EvidenceActivityID string   `json:"evidence_activity_id"`
	EvidenceText       string   `json:"evidence_text"`
	Confidence         *float64 `json:"confidence"`
}

type stalledDraftSignal struct {
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
	Confidence *float64 `json:"confidence"`
}

type stalledEvidenceCandidate struct {
	stalledEvidenceSignal
	Source string
}

type stalledDraftCandidate struct {
	stalledDraftSignal
	Source string
}

func chooseHighestConfidence[T any](items []T, score func(T) float64) (T, bool) {
	var best T
	var ok bool
	var bestScore float64
	for _, item := range items {
		s := score(item)
		if !ok || s > bestScore {
			best = item
			bestScore = s
			ok = true
		}
	}
	return best, ok
}

// StalledRecoveryProduce scans view.Facts for stalled recovery facts and emits
// at most one staged recovery proposal per deal. The producer never invents a
// reason, evidence, or draft; malformed claim/evidence input is skipped.
func StalledRecoveryProduce(view domain.AssembledView) ([]domain.Proposal, error) {
	claims, evidence, drafts := parseStalledRecoveryFacts(view.Facts)

	out := make([]domain.Proposal, 0, len(claims))
	processed := map[string]bool{}
	for _, fact := range view.Facts {
		if fact.EntityType != "deal_stalled_claim" {
			continue
		}
		dealID := fact.EntityID
		if processed[dealID] {
			continue
		}
		processed[dealID] = true

		if p, ok := buildStalledRecoveryProposal(view.WorkspaceID, dealID, claims, evidence, drafts); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// parseStalledRecoveryFacts groups view.Facts into per-deal claim/evidence/
// draft candidates, decoding and validating each fact's Detail JSON.
// Malformed or incomplete facts are skipped, not an error.
func parseStalledRecoveryFacts(facts []domain.Fact) (map[string]stalledClaim, map[string][]stalledEvidenceCandidate, map[string][]stalledDraftCandidate) {
	claims := map[string]stalledClaim{}
	evidence := map[string][]stalledEvidenceCandidate{}
	drafts := map[string][]stalledDraftCandidate{}

	for _, fact := range facts {
		switch fact.EntityType {
		case "deal_stalled_claim":
			if claim, ok := decodeStalledClaim(fact.Detail); ok {
				claims[fact.EntityID] = claim
			}
		case "recovery_evidence_signal":
			if sig, ok := decodeStalledEvidenceSignal(fact.Detail); ok {
				evidence[fact.EntityID] = append(evidence[fact.EntityID], stalledEvidenceCandidate{stalledEvidenceSignal: sig, Source: fact.Source})
			}
		case "recovery_draft_signal":
			if sig, ok := decodeStalledDraftSignal(fact.Detail); ok {
				drafts[fact.EntityID] = append(drafts[fact.EntityID], stalledDraftCandidate{stalledDraftSignal: sig, Source: fact.Source})
			}
		}
	}
	return claims, evidence, drafts
}

func decodeStalledClaim(raw string) (stalledClaim, bool) {
	var claim stalledClaim
	if err := json.Unmarshal([]byte(raw), &claim); err != nil {
		return stalledClaim{}, false
	}
	if claim.GenericReason == "" || claim.Confidence == nil {
		return stalledClaim{}, false
	}
	return claim, true
}

func decodeStalledEvidenceSignal(raw string) (stalledEvidenceSignal, bool) {
	var sig stalledEvidenceSignal
	if err := json.Unmarshal([]byte(raw), &sig); err != nil {
		return stalledEvidenceSignal{}, false
	}
	if sig.SpecificReason == "" || sig.EvidenceActivityID == "" || sig.EvidenceText == "" || sig.Confidence == nil {
		return stalledEvidenceSignal{}, false
	}
	return sig, true
}

func decodeStalledDraftSignal(raw string) (stalledDraftSignal, bool) {
	var sig stalledDraftSignal
	if err := json.Unmarshal([]byte(raw), &sig); err != nil {
		return stalledDraftSignal{}, false
	}
	if sig.Subject == "" || sig.Body == "" || sig.Confidence == nil {
		return stalledDraftSignal{}, false
	}
	return sig, true
}

// buildStalledRecoveryProposal assembles the one Proposal for dealID, if a
// well-formed, non-suppressed claim and a matching evidence signal both
// exist. Returns ok=false for a suppressed (wait_until_active), malformed,
// or evidence-less deal — never a fabricated reason/evidence/draft.
func buildStalledRecoveryProposal(workspaceID, dealID string, claims map[string]stalledClaim, evidence map[string][]stalledEvidenceCandidate, drafts map[string][]stalledDraftCandidate) (domain.Proposal, bool) {
	claim, ok := claims[dealID]
	if !ok || claim.WaitUntilActive {
		return domain.Proposal{}, false
	}
	bestEvidence, ok := chooseHighestConfidence(evidence[dealID], func(sig stalledEvidenceCandidate) float64 { return *sig.Confidence })
	if !ok {
		return domain.Proposal{}, false
	}
	var draft map[string]string
	if bestDraft, ok := chooseHighestConfidence(drafts[dealID], func(sig stalledDraftCandidate) float64 { return *sig.Confidence }); ok {
		draft = map[string]string{"subject": bestDraft.Subject, "body": bestDraft.Body}
	}
	effect, err := json.Marshal(stalledRecoveryEffect{
		Reason:             bestEvidence.SpecificReason,
		EvidenceActivityID: bestEvidence.EvidenceActivityID,
		DealID:             dealID,
		WorkspaceID:        workspaceID,
		Draft:              draft,
	})
	if err != nil {
		return domain.Proposal{}, false
	}
	return domain.Proposal{
		WorkspaceID:  workspaceID,
		ActionType:   "stalled_recovery",
		TargetEntity: "deal:" + dealID,
		Effect:       effect,
		Evidence:     fmt.Sprintf("%s: %s", bestEvidence.SpecificReason, bestEvidence.EvidenceText),
		Confidence:   bestEvidence.Confidence,
		Source:       bestEvidence.Source,
	}, true
}

var _ ports.Produce = StalledRecoveryProduce
