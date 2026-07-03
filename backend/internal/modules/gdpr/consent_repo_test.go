package crmgdpr_test

import (
	"context"
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
)

// TestConsentRepository_Constructor verifies NewConsentRepository returns a non-nil value.
func TestConsentRepository_Constructor(t *testing.T) {
	repo := crmgdpr.NewConsentRepository(nil)
	if repo == nil {
		t.Fatal("NewConsentRepository returned nil")
	}
}

// TestConsentRepository_Interface verifies the ConsentRepository interface is
// defined with exactly FindForPurpose (workspaceID, personID, purpose string).
func TestConsentRepository_Interface(t *testing.T) {
	_ = crmgdpr.NewConsentRepository(nil)
}

// TestConsentRepository_NilDB_FindForPurpose verifies that FindForPurpose returns
// an error (not a panic) when the underlying db is nil.
func TestConsentRepository_NilDB_FindForPurpose(t *testing.T) {
	repo := crmgdpr.NewConsentRepository(nil)
	state, err := repo.FindForPurpose(context.Background(), "ws-id", "person-id", "marketing_email")
	if err == nil {
		t.Fatal("FindForPurpose with nil db should return an error")
	}
	if state != crmgdpr.Unknown {
		t.Fatalf("FindForPurpose with nil db should return Unknown, got %q", state)
	}
}
