package crmgdpr

import (
	"testing"
)

// TestBuildErasureTombstone_PIIFree verifies the PII-free contract of the tombstone:
// Before and After must always be nil, and Action/EntityType must be correct.
func TestBuildErasureTombstone_PIIFree(t *testing.T) {
	pid := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	entry := buildErasureTombstone(pid)

	if entry.Action != "erase" {
		t.Errorf("Action: want erase, got %q", entry.Action)
	}
	if entry.EntityType != "person" {
		t.Errorf("EntityType: want person, got %q", entry.EntityType)
	}
	if entry.EntityID == nil || *entry.EntityID != pid {
		t.Errorf("EntityID: want %q, got %v", pid, entry.EntityID)
	}
	if entry.Before != nil {
		t.Errorf("Before must be nil (PII-free tombstone), got %v", entry.Before)
	}
	if entry.After != nil {
		t.Errorf("After must be nil (PII-free tombstone), got %v", entry.After)
	}
	if entry.AuthorizationRule == nil || *entry.AuthorizationRule == "" {
		t.Error("AuthorizationRule must be set")
	}
}
