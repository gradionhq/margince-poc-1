package app

import (
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

func produceIntegrityFlags(
	view domain.AssembledView,
	claimEntityType string,
	corroborationEntityType string,
	actionType string,
	checkName string,
	evidence string,
	decode func(string) (string, *float64, bool),
) []domain.Proposal {
	corroborated := map[string]bool{}
	for _, f := range view.Facts {
		if f.EntityType == corroborationEntityType {
			corroborated[f.EntityID] = true
		}
	}

	var out []domain.Proposal
	for _, f := range view.Facts {
		if f.EntityType != claimEntityType {
			continue
		}
		description, confidence, ok := decode(f.Detail)
		if !ok || corroborated[f.EntityID] {
			continue
		}
		effect, _ := json.Marshal(map[string]string{
			"check": checkName,
			"claim": description,
		})
		out = append(out, domain.Proposal{
			WorkspaceID:  view.WorkspaceID,
			ActionType:   actionType,
			TargetEntity: f.EntityID,
			Effect:       effect,
			Evidence:     evidence,
			Confidence:   confidence,
			Source:       f.Source,
		})
	}
	return out
}
