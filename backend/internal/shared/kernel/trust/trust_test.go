package trust

import "testing"

func TestTrustTierValues(t *testing.T) {
	if TierT0 != "T0" || TierT1 != "T1" || TierT2 != "T2" {
		t.Fatalf("tier values drifted: %q %q %q", TierT0, TierT1, TierT2)
	}
}

func TestTrustWarningNonEmpty(t *testing.T) {
	if TrustWarning == "" {
		t.Fatal("TrustWarning must be non-empty")
	}
}
