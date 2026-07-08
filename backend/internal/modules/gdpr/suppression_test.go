package crmgdpr_test

import (
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
)

func TestHash_StableAndCaseInsensitive(t *testing.T) {
	h1 := crmgdpr.Hash("Alice@Example.COM")
	h2 := crmgdpr.Hash("alice@example.com")
	h3 := crmgdpr.Hash("  alice@example.com  ")
	if h1 != h2 {
		t.Errorf("Hash case-sensitive: %q != %q", h1, h2)
	}
	if h1 != h3 {
		t.Errorf("Hash whitespace-sensitive: %q != %q", h1, h3)
	}
	if len(h1) != 64 {
		t.Errorf("Hash: want 64 hex chars (sha256), got %d", len(h1))
	}
}

func TestHash_DifferentEmailsDifferentHashes(t *testing.T) {
	if crmgdpr.Hash("a@b.com") == crmgdpr.Hash("c@d.com") {
		t.Error("different emails must produce different hashes")
	}
}
