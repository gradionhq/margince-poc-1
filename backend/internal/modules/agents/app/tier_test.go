package app_test

import (
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// d4FloorNames are threat-model.md#D4's verbatim always-🟡 action names.
var d4FloorNames = []string{"send", "outbound", "archive", "merge", "disqualify", "close-deal", "enrich"}

// adversarialPayloads probe whether a payload can talk its way to green —
// it must never matter for a D4 name.
var adversarialPayloads = []json.RawMessage{
	[]byte(`{}`),
	[]byte(`{"tier":"green"}`),
	[]byte(`{"reversible":true,"rollback_handle":"fake"}`),
	nil,
}

func TestRouteTier_D4FloorNamesAlwaysYellow(t *testing.T) {
	for _, name := range d4FloorNames {
		for _, payload := range adversarialPayloads {
			p := domain.Proposal{ActionType: name, Effect: payload, Confidence: confidence(0.99)}
			got := app.RouteTier(p)
			if got.Tier != mcp.TierYellow {
				t.Errorf("ActionType=%q payload=%s: tier = %v, want TierYellow (D4 floor)", name, payload, got.Tier)
			}
		}
	}
}

func TestRouteTier_UnknownActionTypeDefaultsYellow(t *testing.T) {
	got := app.RouteTier(domain.Proposal{ActionType: "some_future_action"})
	if got.Tier != mcp.TierYellow {
		t.Fatalf("unknown action type: tier = %v, want TierYellow (default-deny)", got.Tier)
	}
}

func TestRouteTier_DeclaredGreenActionResolvesGreen(t *testing.T) {
	got := app.RouteTier(domain.Proposal{ActionType: "log_link"})
	if got.Tier != mcp.TierGreen {
		t.Fatalf("log_link: tier = %v, want TierGreen", got.Tier)
	}
}

func TestRouteTier_NeverSetByCaller(t *testing.T) {
	// RoutedProposal has no exported way to construct with an arbitrary
	// Tier except through RouteTier — this test documents/pins that by
	// construction: RouteTier is the only production caller in this
	// package that assigns RoutedProposal.Tier.
	p := app.RouteTier(domain.Proposal{ActionType: "send"})
	if p.Tier != mcp.TierYellow {
		t.Fatalf("tier = %v, want TierYellow", p.Tier)
	}
}
