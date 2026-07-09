package app

import (
	"encoding/json"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// Fact convention for the stage-unsupported check:
//   - EntityType = "stage_claim": the deal's currently-claimed stage.
//     Detail: {"confidence": <float>, "stage": "<string>"} - both
//     required.
//   - EntityType = "stage_signal": an independently-captured signal
//     about what stage the deal is actually in. Detail:
//     {"confidence": <float>, "supports_stage": "<string>"} - both
//     required for a signal to be usable as a correction source; a
//     malformed stage_signal fact is simply invisible to this check -
//     never counted as support, never a correction source.

type stageClaimDetail struct {
	Confidence *float64 `json:"confidence"`
	Stage      string   `json:"stage"`
}

type stageSignalDetail struct {
	Confidence    *float64 `json:"confidence"`
	SupportsStage string   `json:"supports_stage"`
}

type stageSignal struct {
	detail stageSignalDetail
	source string
}

// ProduceStageUnsupportedFlags emits an "integrity_flag" for every
// well-formed "stage_claim" no same-EntityID "stage_signal" supports,
// and additionally a separate "stage_correction" proposal when at least
// one well-formed "stage_signal" exists for that EntityID (sourced from
// the highest-confidence one - never the claim's own confidence). A
// stage_claim a signal DOES support produces neither. Order-preserving,
// flag emitted before its correction within one claim. Never errors
// (see producer_integrity_check.go).
func ProduceStageUnsupportedFlags(view domain.AssembledView) ([]domain.Proposal, error) {
	signalsByEntity := map[string][]stageSignal{}
	for _, f := range view.Facts {
		if f.EntityType != "stage_signal" {
			continue
		}
		var d stageSignalDetail
		if err := json.Unmarshal([]byte(f.Detail), &d); err != nil {
			continue
		}
		if d.Confidence == nil || d.SupportsStage == "" {
			continue
		}
		signalsByEntity[f.EntityID] = append(signalsByEntity[f.EntityID], stageSignal{detail: d, source: f.Source})
	}

	var out []domain.Proposal
	for _, f := range view.Facts {
		if f.EntityType != "stage_claim" {
			continue
		}
		claim, ok := decodeStageClaim(f.Detail)
		if !ok {
			continue
		}

		signals := signalsByEntity[f.EntityID]
		if stageSupported(signals, claim.Stage) {
			continue
		}

		flagEffect, _ := json.Marshal(map[string]string{
			"check": "stage_unsupported",
			"claim": claim.Stage,
		})
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "integrity_flag",
			TargetEntity: f.EntityID,
			Effect:       flagEffect,
			Evidence:     fmt.Sprintf("captured stage signals do not support the claimed stage %q", claim.Stage),
			Confidence:   claim.Confidence,
			Source:       f.Source,
		})

		if len(signals) == 0 {
			continue
		}
		best := bestStageSignal(signals)
		correctionEffect, _ := json.Marshal(map[string]string{
			"field": "stage",
			"value": best.detail.SupportsStage,
		})
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   "stage_correction",
			TargetEntity: f.EntityID,
			Effect:       correctionEffect,
			Evidence:     fmt.Sprintf("captured stage_signal indicates stage %q", best.detail.SupportsStage),
			Confidence:   best.detail.Confidence,
			Source:       best.source,
		})
	}
	return out, nil
}

func decodeStageClaim(raw string) (stageClaimDetail, bool) {
	var d stageClaimDetail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return stageClaimDetail{}, false
	}
	if d.Confidence == nil || d.Stage == "" {
		return stageClaimDetail{}, false
	}
	return d, true
}

func stageSupported(signals []stageSignal, claimedStage string) bool {
	for _, s := range signals {
		if s.detail.SupportsStage == claimedStage {
			return true
		}
	}
	return false
}

func bestStageSignal(signals []stageSignal) stageSignal {
	best := signals[0]
	for _, s := range signals[1:] {
		if *s.detail.Confidence > *best.detail.Confidence {
			best = s
		}
	}
	return best
}
