package crmgdpr

import (
	"os"
	"regexp"
	"testing"
)

// TestNoRawSetConfig proves GH-209 WS-A's fix: none of gdpr's four historically-broken
// call sites may contain a literal `set_config('app.workspace_id'` any more — that
// statement now lives solely inside platform/database's seam, and every call site here
// must route through it (database.WithWorkspaceTx or database.SetWorkspaceScope) instead
// of opening its own tx and setting the GUC without an accompanying role switch.
func TestNoRawSetConfig(t *testing.T) {
	files := []string{"erasure.go", "sar.go", "evaluator.go", "consent.go"}
	pattern := regexp.MustCompile(`set_config\(\s*'app\.workspace_id'`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if pattern.Match(b) {
			t.Errorf("%s still contains a raw set_config('app.workspace_id' call — route it through platform/database instead", f)
		}
	}
}
