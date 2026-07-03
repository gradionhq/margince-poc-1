package crmcore

import "testing"

func TestTerminalStageTier(t *testing.T) {
	cases := []struct {
		semantic string
		want     stageTier
	}{
		{"open", tierGreen},
		{"won", tierYellow},
		{"lost", tierYellow},
	}
	for _, c := range cases {
		got := terminalStageTier(c.semantic)
		if got != c.want {
			t.Errorf("terminalStageTier(%q) = %v, want %v", c.semantic, got, c.want)
		}
	}
}
