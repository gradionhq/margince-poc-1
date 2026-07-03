package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompositionBothConfigs builds and runs the server binary in two tag configurations
// and asserts the expected jurisdiction set without requiring any live infrastructure
// (the -print-jurisdictions probe exits before any DB/Redis dial).
func TestCompositionBothConfigs(t *testing.T) {
	t.Run("DACH build includes de", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "server-dach")
		if err := exec.Command("go", "build", "-o", bin, ".").Run(); err != nil {
			t.Fatalf("go build DACH: %v", err)
		}
		out, err := exec.Command(bin, "-print-jurisdictions").Output()
		if err != nil {
			t.Fatalf("run DACH: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		found := false
		for _, l := range lines {
			if l == "de" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DACH build: expected 'de' in output, got: %q", string(out))
		}
	})

	t.Run("nopacks build omits de", func(t *testing.T) {
		bin := filepath.Join(t.TempDir(), "server-nopacks")
		if err := exec.Command("go", "build", "-tags", "nopacks", "-o", bin, ".").Run(); err != nil {
			t.Fatalf("go build nopacks: %v", err)
		}
		out, err := exec.Command(bin, "-print-jurisdictions").Output()
		if err != nil {
			t.Fatalf("run nopacks: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, l := range lines {
			if l == "de" {
				t.Errorf("nopacks build: unexpected 'de' in output, got: %q", string(out))
				break
			}
		}
	})
}
