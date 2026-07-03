package main

import "testing"

// TestVersionVarsExist ensures the build-time ldflags targets are declared.
// The defaults ("dev", "unknown", "unknown") are what you get without -ldflags.
func TestVersionVarsExist(t *testing.T) {
	if Version == "" {
		t.Error("Version must not be empty string; default should be \"dev\"")
	}
	if Commit == "" {
		t.Error("Commit must not be empty string; default should be \"unknown\"")
	}
	if BuildDate == "" {
		t.Error("BuildDate must not be empty string; default should be \"unknown\"")
	}
}
