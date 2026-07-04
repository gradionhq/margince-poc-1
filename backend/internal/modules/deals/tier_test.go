package deals

import "testing"

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
