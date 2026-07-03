package crmcore

import "time"

// OrgStrengthInput is one person's contribution to PO-N-ORGSTRENGTH's org
// roll-up. Strength is nil for a no-signal-yet person, so it is excluded from
// the max rather than treated as zero. LastInteraction is only used for
// tie-breaking among people with a computed strength.
type OrgStrengthInput struct {
	PersonID        string
	Strength        *StrengthResult
	LastInteraction time.Time
}

// OrgStrength implements PO-N-ORGSTRENGTH: plain max over the org's people's
// strengths, with no cap and no normalization. Ties resolve by most recent
// LastInteraction, then lowest PersonID. hasSignal is false when every person is
// no-signal-yet.
func OrgStrength(people []OrgStrengthInput) (score int, topPersonID string, hasSignal bool) {
	var best *OrgStrengthInput
	for i := range people {
		p := &people[i]
		if p.Strength == nil {
			continue
		}
		switch {
		case best == nil:
			best = p
		case p.Strength.Score > best.Strength.Score:
			best = p
		case p.Strength.Score == best.Strength.Score:
			if p.LastInteraction.After(best.LastInteraction) {
				best = p
			} else if p.LastInteraction.Equal(best.LastInteraction) && p.PersonID < best.PersonID {
				best = p
			}
		}
	}
	if best == nil {
		return 0, "", false
	}
	return best.Strength.Score, best.PersonID, true
}
