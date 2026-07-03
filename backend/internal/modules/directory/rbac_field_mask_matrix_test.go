//go:build integration

package crmcore_test

import (
	"testing"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
)

// TestRBACFieldMaskMatrix verifies that ValidatePermissions + AuthorizePerms
// correctly represent which objects/actions a role may access (the per-object
// decision is made by the real crmauth functions, not a copy of the logic).
func TestRBACFieldMaskMatrix(t *testing.T) {
	cases := []struct {
		name        string
		permsJSON   map[string]any
		object      string
		action      string
		wantAllowed bool
	}{
		{
			name: "admin/person/read allowed",
			permsJSON: map[string]any{
				"person": map[string]any{"read": map[string]any{"row_scope": "all"}},
			},
			object: "person", action: "read", wantAllowed: true,
		},
		{
			name: "read_only/person/create denied",
			permsJSON: map[string]any{
				"person": map[string]any{"read": map[string]any{"row_scope": "all"}},
			},
			object: "person", action: "create", wantAllowed: false,
		},
		{
			name: "ops/person/read denied (ops has no person)",
			permsJSON: map[string]any{
				"report": map[string]any{"read": map[string]any{"row_scope": "all"}},
			},
			object: "person", action: "read", wantAllowed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			perms, err := crmauth.ValidatePermissions(tc.permsJSON)
			if err != nil {
				t.Fatalf("ValidatePermissions: %v", err)
			}
			err = crmauth.AuthorizePerms(perms, tc.object, tc.action)
			allowed := err == nil
			if allowed != tc.wantAllowed {
				t.Errorf("want allowed=%v, got allowed=%v (err=%v)", tc.wantAllowed, allowed, err)
			}
		})
	}
}
