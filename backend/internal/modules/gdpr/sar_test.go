package crmgdpr_test

import (
	"encoding/json"
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
)

// TestSARPackage_ShapeFromFixture verifies that the SARPackage struct covers all linked stores.
func TestSARPackage_ShapeFromFixture(t *testing.T) {
	pkg := crmgdpr.SARPackage{
		Person:        json.RawMessage(`{"id":"abc","full_name":"Alice"}`),
		Emails:        []json.RawMessage{json.RawMessage(`{"email":"alice@example.com"}`)},
		Activities:    []json.RawMessage{json.RawMessage(`{"subject":"Meeting"}`)},
		Deals:         []json.RawMessage{json.RawMessage(`{"name":"Deal A"}`)},
		Organizations: []json.RawMessage{json.RawMessage(`{"name":"Org A"}`)},
		RawCapture:    []json.RawMessage{json.RawMessage(`{"source":"hubspot"}`)},
	}

	if len(pkg.Person) == 0 {
		t.Error("Person must not be empty")
	}
	if len(pkg.Emails) != 1 {
		t.Errorf("Emails: want 1, got %d", len(pkg.Emails))
	}
	if len(pkg.Activities) != 1 {
		t.Errorf("Activities: want 1, got %d", len(pkg.Activities))
	}
	if len(pkg.Deals) != 1 {
		t.Errorf("Deals: want 1, got %d", len(pkg.Deals))
	}
	if len(pkg.Organizations) != 1 {
		t.Errorf("Organizations: want 1, got %d", len(pkg.Organizations))
	}
	if len(pkg.RawCapture) != 1 {
		t.Errorf("RawCapture: want 1, got %d", len(pkg.RawCapture))
	}
}
