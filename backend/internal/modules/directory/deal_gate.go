package crmcore

// stageTier classifies a stage's approval requirement level.
type stageTier int

const (
	tierGreen  stageTier = iota // no approval required
	tierYellow                  // approval required for agent callers
)

// terminalStageTier returns tierYellow for terminal semantics (won, lost) and tierGreen otherwise.
func terminalStageTier(semantic string) stageTier {
	switch semantic {
	case statusWon, statusLost:
		return tierYellow
	default:
		return tierGreen
	}
}
