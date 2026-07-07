package crmapprovals

import (
	"os"
	"regexp"
	"testing"
)

// TestDeciderUsesSetWorkspaceScope proves decision.go's three methods no
// longer issue a bare `set_config` without an adjacent role switch (GH-209
// WS-A design deviation D1) — they must call database.SetWorkspaceScope on
// their caller-supplied tx instead.
func TestDeciderUsesSetWorkspaceScope(t *testing.T) {
	b, err := os.ReadFile("app/decision.go")
	if err != nil {
		t.Fatalf("read app/decision.go: %v", err)
	}
	if regexp.MustCompile(`set_config\(\s*'app\.workspace_id'`).Match(b) {
		t.Error("decision.go still contains a raw set_config('app.workspace_id' call — use database.SetWorkspaceScope")
	}
	if !regexp.MustCompile(`database\.SetWorkspaceScope`).Match(b) {
		t.Error("decision.go does not call database.SetWorkspaceScope — Approve/Reject/Modify must scope their caller-supplied tx")
	}
}
