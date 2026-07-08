package app

// package app (white-box), not app_test: entityIDFromTarget is unexported,
// mirroring entityTypeFromTarget — there is no exported seam to reach it
// from app_test, so this table-driven test lives alongside the code it
// covers instead.

import "testing"

func TestEntityIDFromTarget(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{name: "kind and id", target: "deal:abc", want: "abc"},
		{name: "no colon", target: "no-colon-here", want: "no-colon-here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entityIDFromTarget(tc.target); got != tc.want {
				t.Fatalf("entityIDFromTarget(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}
