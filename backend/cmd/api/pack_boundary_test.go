package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	coreModPkg = "github.com/gradionhq/margince/backend/internal/modules/people"
	packMod    = "github.com/gradionhq/margince/jurisdictions/de"
)

// TestCoreImportsNoPack proves the cross-module boundary go-arch-lint can't model:
// a representative core package's full dependency closure contains no pack path.
// (crm-de requires crm one-way, so crm -> crm-de is a compile-impossible cycle.)
func TestCoreImportsNoPack(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", coreModPkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", coreModPkg, err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), packMod) {
			t.Errorf("core package %s transitively imports pack %s", coreModPkg, line)
		}
	}
}

// TestOnlyCompositionRootImportsPack proves cmd/server (default tags) pulls in the
// pack while a -tags nopacks build does not. Complements composition_test.go's
// behavioral probe with a dependency-graph assertion.
func TestOnlyCompositionRootImportsPack(t *testing.T) {
	withPack, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps . (default): %v\n%s", err, withPack)
	}
	if !strings.Contains(string(withPack), packMod) {
		t.Errorf("default cmd/server build should import pack %s; not found in deps", packMod)
	}
	noPack, err := exec.Command("go", "list", "-deps", "-tags", "nopacks", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -tags nopacks .: %v\n%s", err, noPack)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(noPack)), "\n") {
		if strings.TrimSpace(line) == packMod {
			t.Errorf("nopacks build should NOT import pack %s, but it appears in deps", packMod)
		}
	}
}

// archLintBin locates the go-arch-lint binary the Makefile uses, or "" if absent.
func archLintBin() string {
	if p, err := exec.LookPath("go-arch-lint"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, "go", "bin", "go-arch-lint")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// TestArchLintRejectsForbiddenEdge proves the intra-module DAG gate has teeth via a
// hermetic fixture mini-module: component "a" is not allowed to depend on "b", but
// a Go file in "a" imports "b". go-arch-lint check must exit non-zero. A clean
// variant (import removed) must exit zero. Real crm source is never touched.
func TestArchLintRejectsForbiddenEdge(t *testing.T) {
	bin := archLintBin()
	if bin == "" {
		t.Skip("go-arch-lint not installed (run make tools); skipping teeth test")
	}
	dir := t.TempDir()
	mod := "example.com/fixture"
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module "+mod+"\n\ngo 1.26\n")
	write("b/b.go", "package b\n\nfunc B() {}\n")
	// .go-arch-lint v3: a is not allowed to import any internal component (b is
	// absent from its mayDependOn). anyVendorDeps: true satisfies the validator's
	// "must have at least one ref or flag" requirement while keeping the internal-dep
	// restriction in place — an empty mayDependOn: [] is rejected by go-arch-lint.
	write(".go-arch-lint.yml", `version: 3
allow:
  deepScan: false
components:
  a: { in: a }
  b: { in: b }
deps:
  a:
    anyVendorDeps: true
  b:
    anyVendorDeps: true
`)

	run := func() (bool, string) {
		cmd := exec.Command(bin, "check")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return err == nil, string(out)
	}

	// Violating variant: a imports b (forbidden).
	write("a/a.go", "package a\n\nimport _ \""+mod+"/b\"\n\nfunc A() {}\n")
	ok, out := run()
	if ok {
		t.Fatalf("go-arch-lint should REJECT a->b forbidden edge, but it passed:\n%s", out)
	}

	// Clean variant: drop the forbidden import.
	write("a/a.go", "package a\n\nfunc A() {}\n")
	ok, out = run()
	if !ok {
		t.Fatalf("go-arch-lint should accept the clean fixture, but it failed:\n%s", out)
	}
}
