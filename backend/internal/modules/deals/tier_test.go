package deals

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

func TestResolveTier_EitherEndpointTerminal(t *testing.T) {
	cases := []struct {
		name, from, to string
		want           Tier
	}{
		{"open to open", "open", "open", TierGreen},
		{"open to won (close)", "open", "won", TierYellow},
		{"open to lost (close)", "open", "lost", TierYellow},
		{"won to open (reopen)", "won", "open", TierYellow},
		{"lost to open (reopen)", "lost", "open", TierYellow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveTier(c.from, c.to); got != c.want {
				t.Errorf("ResolveTier(%q,%q) = %v, want %v", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestResolveTier_IsSemanticNotNameDriven(t *testing.T) {
	if ResolveTier("open", "won") != TierYellow {
		t.Fatal("a terminal semantic must always yield TierYellow regardless of naming")
	}
}

func TestResolveDynamicTier_AdaptsToggleGateArgsToMCPRiskTier(t *testing.T) {
	if got := ResolveDynamicTier(map[string]any{"from_semantic": "open", "to_semantic": "open"}); got != mcp.TierGreen {
		t.Fatalf("open to open = %v, want mcp.TierGreen", got)
	}
	if got := ResolveDynamicTier(map[string]any{"from_semantic": "open", "to_semantic": "won"}); got != mcp.TierYellow {
		t.Fatalf("open to won = %v, want mcp.TierYellow", got)
	}
	if got := ResolveDynamicTier(map[string]any{}); got != mcp.TierGreen {
		t.Fatalf("missing keys = %v, want mcp.TierGreen (empty semantics are non-terminal)", got)
	}
}
