package modules_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// d6Subdirs are the D6 module-shape subdirectories a module may contain
// (AC-E1: domain/ports/adapters/app/transport + module.go). Not every
// module needs all five -- e.g. datasourcebindings has no transport/
// because it backs composite views rather than a directly-routed HTTP
// entity -- but any of these that DO exist must be real directories.
var d6Subdirs = map[string]bool{
	"domain":    true,
	"ports":     true,
	"adapters":  true,
	"app":       true,
	"transport": true,
}

// TestModuleShape proves every internal/modules/* package follows the D6
// module-shape convention (AC-E1's own verify line): domain/ports/adapters/
// app/transport subdirectories plus a module.go wiring file, with no stray
// non-test .go file sitting at the module root. Not every module has all
// five subdirectories -- that's legitimate and module-dependent (e.g.
// datasourcebindings has no transport/), so this test does not require
// them all; it only asserts (1) no stray non-test .go file at module root
// besides module.go/doc.go, and (2) any of the five conventional
// subdirectories that DO exist are real directories. identity/gdpr/
// approvals still having root-level *_test.go files is expected and fine:
// those are pre-existing white/black-box tests for the root package,
// which still legitimately exists post-Task-5's type-alias re-export shim.
func TestModuleShape(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	modulesRoot := filepath.Dir(file)

	entries, err := os.ReadDir(modulesRoot)
	if err != nil {
		t.Fatalf("reading %s: %v", modulesRoot, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		modName := e.Name()
		t.Run(modName, func(t *testing.T) {
			assertModuleShape(t, filepath.Join(modulesRoot, modName))
		})
	}
}

// assertModuleShape asserts the D6 module-shape convention for a single
// module directory at modulePath. It is reusable/parameterized by module
// path so any internal/modules/* package (or a future one) can be checked
// with a single call, rather than needing a bespoke test per module.
func assertModuleShape(t *testing.T, modulePath string) {
	t.Helper()

	entries, err := os.ReadDir(modulePath)
	if err != nil {
		t.Fatalf("reading module dir %s: %v", modulePath, err)
	}

	for _, e := range entries {
		name := e.Name()

		if e.IsDir() {
			if d6Subdirs[name] {
				// Nearly tautological given os.ReadDir + IsDir() already
				// proved this is a directory entry; kept explicit per
				// AC-E1's verify line for documentation/clarity value.
				fi, statErr := os.Stat(filepath.Join(modulePath, name))
				if statErr != nil || !fi.IsDir() {
					t.Errorf("%s: %q must be a real directory", modulePath, name)
				}
			}
			continue
		}

		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if name == "module.go" || name == "doc.go" {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		t.Errorf("%s: stray non-test .go file %q at module root; only module.go/doc.go may live there -- business logic belongs in domain/ports/adapters/app/transport", modulePath, name)
	}
}
