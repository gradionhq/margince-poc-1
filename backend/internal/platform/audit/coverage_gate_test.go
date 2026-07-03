package crmaudit_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAuditCoverageGate_FailsOnBypass builds a throwaway repo tree with a
// bypass mutation and asserts the gate script exits non-zero.
//
// Path depth note (1c restructure, task-3-brief.md): this package moved from
// internal/crm-audit (2 segments under backend/) to internal/platform/audit
// (3 segments), one level deeper — the "../scripts/..." resolution and the
// fixture's synthetic scan-root tree (now backend/internal/modules/directory,
// matching check-audit-coverage.sh's Rule 2 scan root) both updated to match.
func TestAuditCoverageGate_FailsOnBypass(t *testing.T) {
	script, err := filepath.Abs("../../../../scripts/check-audit-coverage.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("gate script not present: %v", err)
	}
	dir := t.TempDir()
	core := filepath.Join(dir, "backend", "internal", "modules", "directory")
	if err := os.MkdirAll(core, 0o755); err != nil {
		t.Fatal(err)
	}
	bypass := "package crmcore\nfunc bad() { _ = `INSERT INTO person (id) VALUES (1)` }\n"
	if err := os.WriteFile(filepath.Join(core, "bypass.go"), []byte(bypass), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, dir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("gate must FAIL on a bypass mutation, but it passed:\n%s", out)
	}
}

// TestAuditCoverageGate_PassesOnRealTree asserts the actual repo passes the gate.
func TestAuditCoverageGate_PassesOnRealTree(t *testing.T) {
	script, _ := filepath.Abs("../../../../scripts/check-audit-coverage.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("gate script not present: %v", err)
	}
	cmd := exec.Command("bash", script, "../../../..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gate must PASS on the real tree, failed:\n%s", out)
	}
}
