package crmgdpr_test

import (
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
)

// TestConsentState_Constants verifies the exported constant values are well-typed
// and distinct — catches accidental typos or collisions.
func TestConsentState_Constants(t *testing.T) {
	states := []crmgdpr.ConsentState{
		crmgdpr.Granted,
		crmgdpr.Withdrawn,
		crmgdpr.Unknown,
	}
	seen := map[crmgdpr.ConsentState]bool{}
	for _, s := range states {
		if string(s) == "" {
			t.Fatalf("ConsentState constant must not be empty string")
		}
		if seen[s] {
			t.Fatalf("duplicate ConsentState value %q", s)
		}
		seen[s] = true
	}
	if crmgdpr.Granted == crmgdpr.Withdrawn {
		t.Fatal("Granted must not equal Withdrawn")
	}
	if crmgdpr.Granted == crmgdpr.Unknown {
		t.Fatal("Granted must not equal Unknown")
	}
}
