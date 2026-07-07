package crmauth

import (
	"os"
	"regexp"
	"testing"
)

// TestNoRawPoolAccessWithoutExemption proves GH-209's escalation fold-in
// (Option 1, approved 2026-07-07): every one of this package's dormant/live
// stores either routes through platform/database, or carries a reasoned
// // rls-exempt: marker on the immediately-preceding line (the same escape
// hatch scripts/check-rls-store-path.sh already honors). This mirrors
// gdpr/approvals/audit's own TestNoRawSetConfig from Tasks 2-3.
func TestNoRawPoolAccessWithoutExemption(t *testing.T) {
	files := []string{
		"oauth_client_store.go", "oauth_auth_code_store.go",
		"connector_secret_store.go", "incumbent_connection_store.go",
		"session_store.go", "passport_store.go",
	}
	pattern := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*\.db\.(ExecContext|QueryContext|QueryRowContext)`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		lines := regexp.MustCompile("\n").Split(string(b), -1)
		for i, line := range lines {
			if !pattern.MatchString(line) {
				continue
			}
			if i == 0 || !regexp.MustCompile(`//\s*rls-exempt:`).MatchString(lines[i-1]) {
				t.Errorf("%s:%d: bare pool access with no adjacent scoping or rls-exempt marker: %s", f, i+1, line)
			}
		}
	}
}
