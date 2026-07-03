package jurisdiction_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findScript walks up from the test's working directory to locate the shipped
// fitness script, so the test works regardless of go test's CWD.
func findScript(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		p := filepath.Join(dir, "scripts", "check-no-jurisdiction.sh")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("check-no-jurisdiction.sh not found; skipping (portable)")
		}
		dir = parent
	}
}

// runScript runs the fitness script against target and returns exit-success + combined output.
func runScript(t *testing.T, script, target string) (bool, string) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable; skipping (portable)")
	}
	out, err := exec.Command("bash", script, target).CombinedOutput()
	return err == nil, string(out)
}

// TestFitnessGateRedOnViolation proves the gate has teeth: a core file with an
// unambiguous country identifier (GoBD) makes it exit non-zero and name the hit.
func TestFitnessGateRedOnViolation(t *testing.T) {
	script := findScript(t)
	dir := t.TempDir()
	core := filepath.Join(dir, "core")
	if err := os.MkdirAll(core, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(core, "x.go"), []byte("package core\nconst note = \"GoBD\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, out := runScript(t, script, dir)
	if ok {
		t.Fatalf("expected non-zero exit on seeded GoBD violation, got success. output:\n%s", out)
	}
	if !strings.Contains(out, "GoBD") || !strings.Contains(out, "x.go") {
		t.Errorf("expected output to name the offending file+string, got:\n%s", out)
	}
}

// TestFitnessGateGreenOnCleanAndSeamExcluded proves no false-positive: a clean
// core file passes, and a jurisdiction/ subdir file containing GoBD is excluded
// by the seam carve-out, so the overall exit is 0.
func TestFitnessGateGreenOnCleanAndSeamExcluded(t *testing.T) {
	script := findScript(t)
	dir := t.TempDir()
	core := filepath.Join(dir, "core")
	juris := filepath.Join(dir, "jurisdiction")
	for _, d := range []string{core, juris} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(core, "clean.go"), []byte("package core\nconst ok = \"hello\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Seam file legitimately contains GoBD — must be excluded.
	if err := os.WriteFile(filepath.Join(juris, "de.go"), []byte("package jurisdiction\nconst fmtName = \"GoBD\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, out := runScript(t, script, dir)
	if !ok {
		t.Fatalf("expected exit 0 on clean+seam-excluded tree, got failure. output:\n%s", out)
	}
}
