package crmgdpr

import "testing"

func TestHash_StableAndCaseInsensitive(t *testing.T) {
	h1 := Hash("Alice@Example.COM")
	h2 := Hash("alice@example.com")
	h3 := Hash("  alice@example.com  ")
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
	if Hash("a@b.com") == Hash("c@d.com") {
		t.Error("different emails must produce different hashes")
	}
}
