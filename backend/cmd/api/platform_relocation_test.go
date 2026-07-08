package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPlatformRelocationLayout asserts the structural outcome of Task 8 (WS-E-d):
//   - internal/platform/{config,events,auth,logger} must all exist
//   - internal/shared/ports/workflow must be gone (types folded into platform/events)
//   - cmd/api must no longer hold outbox_relay.go, river.go, config.go, or redis.go
//     (all moved to internal/platform/*)
func TestPlatformRelocationLayout(t *testing.T) {
	root := repoRoot(t)

	// New platform packages must exist after the relocation.
	for _, pkg := range []string{"config", "events", "auth", "logger"} {
		p := filepath.Join(root, "backend", "internal", "platform", pkg)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("internal/platform/%s must exist after Task 8 platform relocation: %v", pkg, err)
		}
	}

	// workflow port must be gone; its EventEnvelope/Handler types fold into platform/events.
	workflowPath := filepath.Join(root, "backend", "internal", "shared", "ports", "workflow")
	if _, err := os.Stat(workflowPath); err == nil {
		t.Errorf("internal/shared/ports/workflow should be removed after Task 8 (types folded into platform/events)")
	}

	// Files moved out of cmd/api must no longer exist there.
	for _, file := range []string{"outbox_relay.go", "river.go", "config.go", "redis.go"} {
		p := filepath.Join(root, "backend", "cmd", "api", file)
		if _, err := os.Stat(p); err == nil {
			t.Errorf("cmd/api/%s should be removed after Task 8 (moved to internal/platform/*)", file)
		}
	}

	// obs kernel package must be gone; it is reclassified as platform/logger.
	obsPath := filepath.Join(root, "backend", "internal", "shared", "kernel", "obs")
	if _, err := os.Stat(obsPath); err == nil {
		t.Errorf("internal/shared/kernel/obs should be removed after Task 8 (reclassified as internal/platform/logger)")
	}

	// middleware.go must no longer sit inside platform/httpserver; it moved to platform/auth.
	mwPath := filepath.Join(root, "backend", "internal", "platform", "httpserver", "middleware.go")
	if _, err := os.Stat(mwPath); err == nil {
		t.Errorf("internal/platform/httpserver/middleware.go should be removed after Task 8 (moved to internal/platform/auth)")
	}
}
