package transport

import (
	"os"
	"regexp"
	"testing"
)

func TestNoRawSetConfig(t *testing.T) {
	b, err := os.ReadFile("members_handler.go")
	if err != nil {
		t.Fatalf("read members_handler.go: %v", err)
	}
	if regexp.MustCompile(`set_config\(\s*'app\.workspace_id'`).Match(b) {
		t.Error("members_handler.go still contains a raw set_config('app.workspace_id' call — route it through platform/database instead")
	}
}
