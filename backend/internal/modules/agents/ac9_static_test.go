package crmagents_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenEgressImports are packages this leaf module must never import
// directly. Any real outbound send must route through the approvals 🟡
// gate (crmapprovals.Stage) — never a direct SMTP/HTTP/webhook client.
// This is OVN-AC-9's static half: no unattended multi-channel auto-send
// path may ship in this module, and this test fails the build the moment
// one is added, before it can ship (a source-scan, not a runtime check —
// deliberately simple per the spec's own "do not over-engineer" steer).
var forbiddenEgressImports = []string{
	"net/smtp",
	"net/http", // an HTTP client is itself a send/webhook vector this module has no legitimate use for
}

func TestOVNAC9_NoDirectEgressImport(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenEgressImports {
				if importPath == forbidden {
					t.Errorf("%s imports %q directly — all outbound must route through the approvals 🟡 gate (OVN-AC-9)", path, forbidden)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk backend/internal/modules/agents: %v", err)
	}
}
