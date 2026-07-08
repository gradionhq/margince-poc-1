package app

import (
	"sort"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
)

// BuildBatch groups routed proposals by ActionType and ranks each group by
// confidence descending (OVN-AC-3, a defensible documented signal — no
// other ranking factor exists yet since no real producer ships in this
// ticket). producerErr, when non-nil, marks the run degraded regardless of
// how many proposals survived — the run never blocks or silently drops
// the failure (P4); a nil producerErr with zero proposals is the honest
// quiet state, never padded.
func BuildBatch(in []domain.RoutedProposal, producerErr error) domain.RunResult {
	groups := groupAndRank(in)
	switch {
	case producerErr != nil:
		return domain.RunResult{State: domain.RunDegraded, Groups: groups, DegradedReason: producerErr.Error()}
	case len(in) == 0:
		return domain.RunResult{State: domain.RunQuiet}
	default:
		return domain.RunResult{State: domain.RunNormal, Groups: groups}
	}
}

func groupAndRank(in []domain.RoutedProposal) []domain.ProposalGroup {
	byType := map[string][]domain.RoutedProposal{}
	var order []string
	for _, p := range in {
		if _, seen := byType[p.ActionType]; !seen {
			order = append(order, p.ActionType)
		}
		byType[p.ActionType] = append(byType[p.ActionType], p)
	}
	sort.Strings(order) // deterministic group order

	groups := make([]domain.ProposalGroup, 0, len(order))
	for _, actionType := range order {
		items := byType[actionType]
		sort.SliceStable(items, func(i, j int) bool {
			ci, cj := 0.0, 0.0
			if items[i].Confidence != nil {
				ci = *items[i].Confidence
			}
			if items[j].Confidence != nil {
				cj = *items[j].Confidence
			}
			return ci > cj
		})
		groups = append(groups, domain.ProposalGroup{ActionType: actionType, Items: items})
	}
	return groups
}
