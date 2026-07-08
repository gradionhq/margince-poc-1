package httpserver_test

import (
	"os/exec"
	"strings"
	"testing"
)

const (
	httpserverPkg  = "github.com/gradionhq/margince/backend/internal/platform/httpserver"
	identityModule = "github.com/gradionhq/margince/backend/internal/modules/identity"
)

// TestHttpserverNoIdentityDep asserts that platform/httpserver's full
// transitive dependency closure contains no internal/modules/identity path.
// This must FAIL before the port inversion (AC-E3) and PASS after it.
func TestHttpserverNoIdentityDep(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", httpserverPkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", httpserverPkg, err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		dep := strings.TrimSpace(line)
		if strings.HasPrefix(dep, identityModule) {
			t.Errorf("platform/httpserver must not transitively import %s (found: %s); invert via ports/session", identityModule, dep)
		}
	}
}
