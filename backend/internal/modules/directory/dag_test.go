package crmcore

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoreImportsNoConcreteConnector asserts crm-core (Tier-1) never imports a
// concrete connector package (crm-capture/connectors). Connectors reach core
// only through the Tier-0 connector seam (ADR-0014). This complements arch-lint.
func TestCoreImportsNoConcreteConnector(t *testing.T) {
	fset := token.NewFileSet()
	forbidden := "crm-capture/connectors"
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, forbidden) {
				t.Errorf("%s imports forbidden concrete connector %s", e.Name(), imp.Path.Value)
			}
		}
	}
}
