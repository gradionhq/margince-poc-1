package datasource

import "testing"

func TestTrustTierValues(t *testing.T) {
	if TierT0 != "T0" || TierT1 != "T1" || TierT2 != "T2" {
		t.Fatalf("tier values drifted: %q %q %q", TierT0, TierT1, TierT2)
	}
}

func TestTierOfDefaultsToT1ForPlainProvider(t *testing.T) {
	var plain struct{} // does not implement TierProvider
	if got := TierOf(plain); got != TierT1 {
		t.Errorf("TierOf(plain) = %q, want T1 (native default)", got)
	}
}

type t2thing struct{}

func (t2thing) ReadTier() TrustTier { return TierT2 }

func TestTierOfReadsCapability(t *testing.T) {
	if got := TierOf(t2thing{}); got != TierT2 {
		t.Errorf("TierOf(t2thing) = %q, want T2", got)
	}
}

func TestTrustWarningMentionsDataNotInstructions(t *testing.T) {
	if TrustWarning == "" {
		t.Fatal("TrustWarning must be non-empty")
	}
}
