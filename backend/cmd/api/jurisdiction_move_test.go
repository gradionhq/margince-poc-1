package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test's working directory to find the repo root
// (the directory containing go.work).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.work found)")
		}
		dir = parent
	}
}

// TestJurisdictionSeamMoved asserts that the seam package lives at its new
// canonical path (backend/pkg/shared/ports/jurisdiction) and that the old
// location (backend/pkg/jurisdiction) is gone.
func TestJurisdictionSeamMoved(t *testing.T) {
	root := repoRoot(t)

	oldPath := filepath.Join(root, "backend", "pkg", "jurisdiction")
	if _, err := os.Stat(oldPath); err == nil {
		t.Errorf("old seam path still exists: %s (should have been moved to backend/pkg/shared/ports/jurisdiction)", oldPath)
	}

	newPath := filepath.Join(root, "backend", "pkg", "shared", "ports", "jurisdiction")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new seam path does not exist: %s — move backend/pkg/jurisdiction here", newPath)
	}

	seamFile := filepath.Join(newPath, "jurisdiction.go")
	if _, err := os.Stat(seamFile); err != nil {
		t.Errorf("jurisdiction.go not found at new path %s", seamFile)
	}
}

// TestCrmDeMoved asserts that the DE pack lives at jurisdictions/de (its new
// canonical location) and that the old crm-de/ directory at repo root is gone.
func TestCrmDeMoved(t *testing.T) {
	root := repoRoot(t)

	oldPath := filepath.Join(root, "crm-de")
	if _, err := os.Stat(oldPath); err == nil {
		t.Errorf("old crm-de/ directory still exists at repo root %s (should have been moved to jurisdictions/de/)", oldPath)
	}

	newPath := filepath.Join(root, "jurisdictions", "de")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new jurisdictions/de/ directory does not exist at %s", newPath)
	}

	goMod := filepath.Join(newPath, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		t.Errorf("jurisdictions/de/go.mod not found — it must have its own go.mod as a separate module")
	}
}

// TestGoWorkWiredForJurisdictionsDe asserts that go.work references ./jurisdictions/de
// (not the old ./crm-de) so the workspace build picks up the moved module.
func TestGoWorkWiredForJurisdictionsDe(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		t.Fatalf("reading go.work: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "./crm-de") {
		t.Errorf("go.work still references ./crm-de — update to ./jurisdictions/de")
	}
	if !strings.Contains(content, "./jurisdictions/de") {
		t.Errorf("go.work does not reference ./jurisdictions/de — add it to the use block")
	}
}

// TestCarveoutMatchesNewSeamPath asserts that the fitness script's /jurisdiction/
// carve-out substring still appears in the new seam path
// (backend/pkg/shared/ports/jurisdiction/), so jurisdiction-pack files remain
// excluded from the core-string gate after the move.
func TestCarveoutMatchesNewSeamPath(t *testing.T) {
	root := repoRoot(t)
	scriptPath := filepath.Join(root, "scripts", "check-no-jurisdiction.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading check-no-jurisdiction.sh: %v", err)
	}

	// Confirm the script uses /jurisdiction/ as its carve-out.
	const carveout = "/jurisdiction/"
	if !strings.Contains(string(data), carveout) {
		t.Fatalf("script does not contain carve-out pattern %q — update this test if the carve-out changed", carveout)
	}

	// Confirm the new seam path contains the carve-out substring, so the
	// grep -v '/jurisdiction/' in the script correctly excludes seam files.
	newSeamPath := "backend/pkg/shared/ports/jurisdiction/jurisdiction.go"
	if !strings.Contains(newSeamPath, carveout) {
		t.Errorf("new seam path %q does not contain carve-out substring %q — the script would no longer exclude seam files; either rename the directory to keep 'jurisdiction' in the path or update the script's carve-out pattern in lockstep", newSeamPath, carveout)
	}
}
