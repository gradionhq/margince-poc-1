package mcp

import "testing"

func TestTiers(t *testing.T) {
	if Green == Yellow {
		t.Fatal("tiers must differ")
	}
	// Green/Yellow are the deprecated spellings of the unified RiskTier vocabulary:
	// the same underlying type and values, so a Tool.Tier() result is comparable to
	// a ToolSpec.Tier with no conversion.
	if Green != TierGreen || Yellow != TierYellow {
		t.Fatalf("Green/Yellow must alias TierGreen/TierYellow: Green=%v Yellow=%v", Green, Yellow)
	}
}

// TestResolveTier_FloorClampDynamic proves the 🟡 floor is enforced in code, not
// just documented. A dynamic resolver may resolve a reversible call to a clean 🟢,
// but ONLY a clean TierGreen escapes the gate: a resolver that hands back the
// un-resolved TierDynamic sentinel (or any non-Green value) for a terminal action
// is floored to 🟡, so a mis-classified irreversible action can never relax below
// the approval gate (mcp-surface "floor can never relax to 🟢").
func TestResolveTier_FloorClampDynamic(t *testing.T) {
	// A resolver that fails to concretely classify a terminal action (returns the
	// TierDynamic sentinel instead of 🟡) must floor to 🟡, not slip through as 🟢.
	rogue := ToolSpec{
		Name: "advance_deal", Version: "v1", Tier: TierDynamic,
		Resolver: func([]byte) RiskTier { return TierDynamic },
	}
	if got := rogue.ResolveTier([]byte(`{"to_status":"won"}`)); got != TierYellow {
		t.Fatalf("an unresolved dynamic tier must be floored to Yellow, got %v", got)
	}
	// A legitimate reversible resolution still passes as 🟢 (open→open).
	ok := ToolSpec{
		Name: "advance_deal", Version: "v1", Tier: TierDynamic,
		Resolver: func([]byte) RiskTier { return TierGreen },
	}
	if got := ok.ResolveTier([]byte(`{"to_status":"qualified"}`)); got != TierGreen {
		t.Fatalf("a clean reversible resolution must stay Green, got %v", got)
	}
}

func TestToolSpecValidate(t *testing.T) {
	valid := ToolSpec{Name: "search_records", Version: "v1", Tier: TierGreen}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid green spec should pass: %v", err)
	}

	dynNoResolver := ToolSpec{Name: "advance_deal", Version: "v1", Tier: TierDynamic}
	if err := dynNoResolver.Validate(); err == nil {
		t.Fatal("TierDynamic without a TierResolver must fail validation")
	}

	dynWithResolver := ToolSpec{
		Name: "advance_deal", Version: "v1", Tier: TierDynamic,
		Resolver: func([]byte) RiskTier { return TierYellow },
	}
	if err := dynWithResolver.Validate(); err != nil {
		t.Fatalf("TierDynamic with resolver should pass: %v", err)
	}

	noName := ToolSpec{Version: "v1", Tier: TierGreen}
	if err := noName.Validate(); err == nil {
		t.Fatal("empty Name must fail validation")
	}
}

func TestToolSpecResolveTier(t *testing.T) {
	spec := ToolSpec{
		Name: "advance_deal", Version: "v1", Tier: TierDynamic,
		Resolver: func(args []byte) RiskTier {
			if string(args) == `{"to_status":"won"}` {
				return TierYellow
			}
			return TierGreen
		},
	}
	if got := spec.ResolveTier([]byte(`{"to_status":"won"}`)); got != TierYellow {
		t.Fatalf("won should resolve yellow, got %v", got)
	}
	if got := spec.ResolveTier([]byte(`{"to_status":"qualified"}`)); got != TierGreen {
		t.Fatalf("open→open should resolve green, got %v", got)
	}
	green := ToolSpec{Name: "read_record", Version: "v1", Tier: TierGreen}
	if got := green.ResolveTier(nil); got != TierGreen {
		t.Fatalf("static green resolves green, got %v", got)
	}
}
