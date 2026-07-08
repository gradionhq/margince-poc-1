package records_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// forbiddenFormulaEvalImports is RD-AC-7's denylist: no known expression-interpreter/formula-
// parser library may be imported anywhere in the Go backend. A formula field's logic is source
// (ADR-0002) -- only display of a code-defined GENERATED column (RD-AC-6) is runtime; the moment
// any of these ships as an import, this test fails the build before it can land
// (docs/product/scope.md NEVER-1: no dynamic-schema/expression-interpreter on the hot path).
var forbiddenFormulaEvalImports = []string{
	"github.com/Knetic/govaluate",
	"github.com/expr-lang/expr",
	"github.com/antonmedv/expr",
	"github.com/google/cel-go/cel",
	"github.com/PaesslerAG/gval",
	"github.com/d5/tengo/v2",
	"github.com/dop251/goja",
	"github.com/robertkrimen/otto",
	"github.com/yuin/gopher-lua",
	"github.com/traefik/yaegi/interp",
}

// forbiddenContractOperationSubstrings names formula-authoring surfaces that must never appear
// in the contract -- defining a *new* formula is a reviewed source change (ADR-0002), never a
// runtime-authored one.
var forbiddenContractOperationSubstrings = []string{
	"createformula", "updateformula", "evaluateformula", "formulaexpression", "formula_expression",
}

// backendRoot resolves to the repo's backend/ directory (three levels above this package:
// backend/internal/modules/records) so the scan below covers the whole Go backend, not just this
// module -- RD-AC-7's "ideally the wider backend" steer.
func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../backend/internal/modules/records/negative_scope_test.go -> backend/
	// (records -> modules -> internal -> backend: three levels up, not four)
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestRDAC7_NoFormulaEvalDependencyInGoMod asserts backend/go.mod never requires a known
// expression-interpreter dependency.
func TestRDAC7_NoFormulaEvalDependencyInGoMod(t *testing.T) {
	root := backendRoot(t)
	goModPath := filepath.Join(root, "go.mod")
	contents, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read %s: %v", goModPath, err)
	}
	for _, forbidden := range forbiddenFormulaEvalImports {
		if strings.Contains(string(contents), forbidden) {
			t.Errorf("backend/go.mod requires forbidden expression-interpreter dependency %q -- "+
				"a formula field's logic must be source (ADR-0002), never a runtime-evaluated "+
				"dependency (RD-AC-7, scope#NEVER-1)", forbidden)
		}
	}
}

// TestRDAC7_NoFormulaEvalImportAnywhereInBackend walks every non-test .go file under backend/ and
// fails if any imports a forbidden expression-interpreter package -- mirrors
// backend/internal/modules/agents/ac9_static_test.go's OVN-AC-9 style, widened to the whole
// backend tree per RD-AC-7's scope.
func TestRDAC7_NoFormulaEvalImportAnywhereInBackend(t *testing.T) {
	root := backendRoot(t)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case "migrations", "seed", "api":
				return filepath.SkipDir // no Go source in these
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenFormulaEvalImports {
				if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
					t.Errorf("%s imports %q -- a formula field's logic must be source (ADR-0002); "+
						"no runtime expression-interpreter may exist on the hot path (RD-AC-7, "+
						"scope#NEVER-1)", path, forbidden)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

// TestRDAC7_NoFormulaAuthoringContractOperation asserts the contract never defines an operation
// that accepts and evaluates a user-supplied formula expression at runtime.
func TestRDAC7_NoFormulaAuthoringContractOperation(t *testing.T) {
	root := backendRoot(t)
	contractPath := filepath.Join(root, "api", "crm.yaml")
	contents, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read %s: %v", contractPath, err)
	}
	lower := strings.ToLower(string(contents))
	for _, forbidden := range forbiddenContractOperationSubstrings {
		if strings.Contains(lower, forbidden) {
			t.Errorf("backend/api/crm.yaml appears to define a formula-authoring surface (%q "+
				"found) -- only display of a code-defined GENERATED column is runtime (RD-AC-7)", forbidden)
		}
	}
}
