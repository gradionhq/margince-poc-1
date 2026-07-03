package model

import (
	"strings"
	"testing"
)

// TestSecretStripper_Union covers the union of secret forms both prior call
// sites (crm-ai and crm-capture) needed: crm-ai's set MUST now catch AWS AKIA
// keys, and crm-capture's set MUST now catch ghp_ and the token:/password:
// colon forms. One canonical stripper redacts them all.
func TestSecretStripper_Union(t *testing.T) {
	s := NewSecretStripper()

	cases := []struct {
		name   string
		in     string
		secret string
	}{
		{"bearer token", "Authorization: Bearer sk-ant-AbC123XyZ end", "sk-ant-AbC123XyZ"},
		{"api_key= value", `api_key="sk-test-987654321000" end`, "sk-test-987654321000"},
		{"api-key= value", "api-key=mykey end", "mykey"},
		{"password= value", "password=hunter2secret end", "hunter2secret"},
		{"password: colon form", "password: hunter2secret end", "hunter2secret"},
		{"token: colon form", "token: ghp_aBcDeF1234567890abcdef end", "ghp_aBcDeF1234567890abcdef"},
		{"github PAT bare", "leaked ghp_aBcDeF1234567890abcdef in body", "ghp_aBcDeF1234567890abcdef"},
		{"sk- token bare", "leaked sk-test123 in body", "sk-test123"},
		{"aws akia key", "key AKIAIOSFODNN7EXAMPLE here", "AKIAIOSFODNN7EXAMPLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := s.Strip(tc.in)
			if strings.Contains(out, tc.secret) {
				t.Errorf("secret %q leaked through stripper: %q", tc.secret, out)
			}
			if !strings.Contains(out, "[REDACTED]") {
				t.Errorf("expected [REDACTED] marker, got %q", out)
			}
		})
	}
}

// TestSecretStripper_KeepsPII guards the A8 decision: the stripper redacts
// credentials only, never pseudonymizes names/emails.
func TestSecretStripper_KeepsPII(t *testing.T) {
	s := NewSecretStripper()
	in := "Contact Jane Doe at jane.doe@acme.com about the Q3 deal"
	out := s.Strip(in)
	if !strings.Contains(out, "jane.doe@acme.com") || !strings.Contains(out, "Jane Doe") {
		t.Errorf("stripper must NOT pseudonymize PII; got %q", out)
	}
}
