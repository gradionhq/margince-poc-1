package prov

import "testing"

func TestByHuman(t *testing.T) {
	if !(Provenance{CapturedBy: "human"}).ByHuman() {
		t.Fatal("expected human provenance")
	}
}
