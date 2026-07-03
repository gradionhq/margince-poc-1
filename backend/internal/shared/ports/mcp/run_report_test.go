package mcp

import "testing"

func TestRunReportTool_RegisteredGreen(t *testing.T) {
	var found Tool
	for _, tool := range All() {
		if tool.Name() == "run_report" {
			found = tool
			break
		}
	}
	if found == nil {
		t.Fatal("run_report tool not found in mcp.All()")
	}
	if got := found.Tier(); got != Green {
		t.Fatalf("run_report tier = %v, want Green (%v)", got, Green)
	}
}
