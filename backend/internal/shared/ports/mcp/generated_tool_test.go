package mcp

import (
	"os"
	"strings"
	"testing"
)

// TestGeneratedToolsMatchesCRMYAMLAnnotationCount is the AC-D1 drift-detection
// proof: it independently counts real `x-mcp-tool:` annotations in the repo's
// backend/api/crm.yaml by a plain line scan (deliberately NOT the YAML-walk
// `crm-gen mcp-tools` itself uses, so this is an oracle, not a copy of the
// generator's logic) and asserts GeneratedTools — the actual generated data —
// has exactly that many entries. Add one annotation without running
// `make gen-mcp-tools` and this test fails.
func TestGeneratedToolsMatchesCRMYAMLAnnotationCount(t *testing.T) {
	data, err := os.ReadFile("../../../../api/crm.yaml")
	if err != nil {
		t.Fatalf("read crm.yaml: %v", err)
	}

	want := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "x-mcp-tool:") {
			want++
		}
	}
	if want == 0 {
		t.Fatal("sanity check failed: found zero `x-mcp-tool:` annotations in crm.yaml")
	}

	if got := len(GeneratedTools); got != want {
		t.Fatalf("GeneratedTools has %d entries, crm.yaml has %d `x-mcp-tool:` annotations — run `make gen-mcp-tools` and commit the result", got, want)
	}
}

// TestGeneratedToolsFieldsPopulated guards against a generator regression that
// emits entries with blank OperationID/Verb — every real annotation carries
// both.
func TestGeneratedToolsFieldsPopulated(t *testing.T) {
	for _, tool := range GeneratedTools {
		if tool.OperationID == "" {
			t.Fatalf("GeneratedTools entry with empty OperationID: %+v", tool)
		}
		if tool.Verb == "" {
			t.Fatalf("GeneratedTools entry %q has empty Verb", tool.OperationID)
		}
	}
}
