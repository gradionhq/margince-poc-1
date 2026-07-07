package approvalsport

import "testing"

func TestHashDiff_Deterministic(t *testing.T) {
	a := HashDiff(map[string]any{"to_stage_id": "s1", "status": "won"})
	b := HashDiff(map[string]any{"status": "won", "to_stage_id": "s1"}) // different key order
	if a != b {
		t.Fatalf("HashDiff must be order-independent: %q != %q", a, b)
	}
	c := HashDiff(map[string]any{"to_stage_id": "s2", "status": "won"})
	if a == c {
		t.Fatal("HashDiff must differ for a different diff")
	}
}
