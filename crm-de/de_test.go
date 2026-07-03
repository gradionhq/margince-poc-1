package crmde

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/gradionhq/margince/backend/pkg/jurisdiction"
)

// compile-time assertion: pack implements jurisdiction.Pack.
var _ jurisdiction.Pack = pack{}

func TestSelfRegistered(t *testing.T) {
	p, ok := jurisdiction.For("de")
	if !ok {
		t.Fatal("de pack did not self-register via init()")
	}
	if p.Code() != "de" {
		t.Errorf("Code() = %q, want \"de\"", p.Code())
	}
}

// TestNoCrmCoreImport asserts that no package in crm-de reaches the crm-core
// successor package (backend/internal/modules/directory, 1c restructure —
// task-3-brief.md), directly or transitively (ADR-0014). crm-de touches core
// data only through the jurisdiction seam. This uses the real `go list -deps`
// build graph (like the sibling archtest fitness checks) rather than a shallow
// filename glob: a glob only sees one subdir level and silently skips a deeper
// package, whereas the dependency closure catches an indirect import too.
func TestNoCrmCoreImport(t *testing.T) {
	const forbidden = "github.com/gradionhq/margince/backend/internal/modules/directory"

	cmd := exec.Command("go", "list", "-deps", "-json", "./...")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, stderr.String())
	}

	// `go list -deps ./...` emits every package in the transitive closure of the
	// crm-de packages as a top-level entry. If crm-core's successor (or a
	// subpackage) appears at all, some crm-de package reaches it directly or
	// indirectly — a violation.
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p struct {
			ImportPath string
		}
		if err := dec.Decode(&p); err != nil {
			t.Fatalf("decode go list json: %v", err)
		}
		if p.ImportPath == forbidden || strings.HasPrefix(p.ImportPath, forbidden+"/") {
			t.Errorf("crm-de's dependency graph reaches forbidden package %s (ADR-0014: crm-de must not import crm-core, directly or transitively)", p.ImportPath)
		}
	}
}
