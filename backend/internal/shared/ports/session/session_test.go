package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// TestValidatePermissions_SeededAdminRolePermissions is a regression guard
// against a whole class of bug: a new platformauth.ObjXxx RBAC constant plus
// a matching seeded permissions key (backend/seed/dev.sql) gets added without
// registering the object in knownObjects above (CF-T03).
//
// That drift is invisible to unit tests that build crmctx.Principal directly,
// because it only bites inside the real ValidatePermissions/LoadRolePermissions
// path — and LoadRolePermissions silently `continue`s past a ValidatePermissions
// error per-role-row, so a single unknown object key doesn't just reject that
// object: it makes the ENTIRE role's permission set evaluate to empty, 403-ing
// every RBAC-gated route for that role, not only the new one. Only a live-stack
// boot caught this the first time.
//
// This test parses the real seeded admin role's permissions JSONB literal out
// of seed/dev.sql and runs it through the real validator end-to-end, so any
// future drift between platformauth's Obj* constants / seeded permission keys
// and session.knownObjects fails loud here instead of silently 403-ing a live
// stack.
func TestValidatePermissions_SeededAdminRolePermissions(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine this test file's own path via runtime.Caller")
	}
	seedPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "seed", "dev.sql")
	raw, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("read %s: %v", seedPath, err)
	}

	// The admin role's permissions JSONB literal is the single-quoted JSON
	// blob immediately following the `'admin', true,` role tuple inside
	// seed/dev.sql's `INSERT INTO role (...) VALUES` statement.
	re := regexp.MustCompile(`(?s)'admin',\s*true,\s*\n\s*'(\{.*?\})'`)
	m := re.FindSubmatch(raw)
	if m == nil {
		t.Fatalf("could not locate the seeded admin role's permissions blob in %s "+
			"(seed format changed? update this test's extraction regex)", seedPath)
	}

	var perms map[string]any
	if err := json.Unmarshal(m[1], &perms); err != nil {
		t.Fatalf("seeded admin permissions blob is not valid JSON: %v", err)
	}
	if len(perms) == 0 {
		t.Fatal("seeded admin permissions blob decoded to an empty map — the extraction regex likely matched the wrong span")
	}

	if _, err := ValidatePermissions(perms); err != nil {
		t.Fatalf("ValidatePermissions rejected the seeded admin role's permissions: %v\n"+
			"this reproduces a class of bug where LoadRolePermissions silently drops the ENTIRE "+
			"admin role's permissions (not just the offending object) because it swallows this "+
			"error per-role-row — check for a new platformauth.ObjXxx / seeded permissions key "+
			"added without a matching entry in session.knownObjects", err)
	}
}
