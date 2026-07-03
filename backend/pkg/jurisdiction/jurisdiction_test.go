package jurisdiction

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakepack implements all Pack methods for registry tests.
type fakepack struct{ code string }

func (f fakepack) Code() string                   { return f.code }
func (fakepack) Fiscal() FiscalFormatter          { return nil }
func (fakepack) Retention() RetentionPolicy       { return nil }
func (fakepack) Conformity() ConformityRegime     { return nil }
func (fakepack) TrustArtifacts() TrustArtifactSet { return nil }
func (fakepack) ExportProfiles() []ExportProfile  { return nil }
func (fakepack) Migrations() fs.FS                { return nil }

// saveRegistry snapshots and clears the global registry; the returned func restores it.
func saveRegistry() func() {
	mu.Lock()
	saved := make(map[string]Pack, len(registry))
	for k, v := range registry {
		saved[k] = v
	}
	registry = map[string]Pack{}
	mu.Unlock()
	return func() {
		mu.Lock()
		registry = saved
		mu.Unlock()
	}
}

func TestImportPurity(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(importPath, "github.com/gradionhq/margince") {
				t.Errorf("%s: forbidden crm-* import %q", filepath.Base(name), importPath)
			}
			first := strings.SplitN(importPath, "/", 2)[0]
			if strings.Contains(first, ".") {
				t.Errorf("%s: forbidden third-party import %q", filepath.Base(name), importPath)
			}
		}
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	restore := saveRegistry()
	defer restore()

	Register("dup1", fakepack{"dup1"})
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on duplicate Register, got none")
			}
		}()
		Register("dup1", fakepack{"dup1"})
	}()
}

func TestForUnregistered(t *testing.T) {
	restore := saveRegistry()
	defer restore()

	if _, ok := For("nothere"); ok {
		t.Error("For returned ok=true for unregistered code")
	}
}

func TestApplicableDE(t *testing.T) {
	restore := saveRegistry()
	defer restore()

	Register("de", fakepack{"de"})
	if _, ok := For("de"); !ok {
		t.Fatal("For returned ok=false after Register")
	}
	packs := Applicable("de")
	if len(packs) != 1 {
		t.Fatalf("Applicable(\"de\"): want 1 pack, got %d", len(packs))
	}
	if packs[0].Code() != "de" {
		t.Errorf("Applicable(\"de\")[0].Code() = %q, want \"de\"", packs[0].Code())
	}
}

func TestApplicableUnknown(t *testing.T) {
	restore := saveRegistry()
	defer restore()

	packs := Applicable("xx")
	if len(packs) != 0 {
		t.Errorf("Applicable(\"xx\"): want empty, got %d packs", len(packs))
	}
}
