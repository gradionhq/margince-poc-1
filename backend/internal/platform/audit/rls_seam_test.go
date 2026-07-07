package crmaudit

import (
	"os"
	"regexp"
	"testing"
)

func TestNoRawSetConfig(t *testing.T) {
	files := []string{"crmaudit.go", "agenttrace.go", "smell.go"}
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
