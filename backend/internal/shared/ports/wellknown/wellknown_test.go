package wellknown

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSecurityTxt_RFC9116(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, SecurityTxtPath, nil)
	w := httptest.NewRecorder()

	NewSecurityTxtHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want %d, got %d", http.StatusOK, w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("content-type: want %q, got %q", "text/plain; charset=utf-8", got)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Contact: "+ContactURI) {
		t.Fatalf("body missing contact line: %q", body)
	}
	if !strings.Contains(body, "Expires: "+ExpiresRFC3339) {
		t.Fatalf("body missing expires line: %q", body)
	}
	if !strings.Contains(body, "Policy: "+PolicyURL) {
		t.Fatalf("body missing policy line: %q", body)
	}
}

func TestExpires_IsFutureRFC3339(t *testing.T) {
	got, err := time.Parse(time.RFC3339, ExpiresRFC3339)
	if err != nil {
		t.Fatalf("parse ExpiresRFC3339: %v", err)
	}
	if !got.After(time.Now()) {
		t.Fatalf("ExpiresRFC3339 must be in the future: %s", got.Format(time.RFC3339))
	}
}

func TestSecurityPolicy_Served(t *testing.T) {
	if PolicyURL != SecurityPolicyPath {
		t.Fatalf("policy url chain broken: want %q, got %q", SecurityPolicyPath, PolicyURL)
	}

	req := httptest.NewRequest(http.MethodGet, SecurityPolicyPath, nil)
	w := httptest.NewRecorder()

	NewSecurityPolicyHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want %d, got %d", http.StatusOK, w.Code)
	}

	body := w.Body.String()
	for _, marker := range []string{
		"Scope",
		"safe-harbor",
		"Acknowledgment",
		"triage",
	} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(marker)) {
			t.Fatalf("body missing %q marker: %q", marker, body)
		}
	}
}

// TestRunbook_SingleSourceContact checks testdata/disclosure-triage-runbook.md,
// a package-local fixture — not docs/, which reorganizes independently of this
// package — so the test doesn't break every time the docs tree is restructured.
func TestRunbook_SingleSourceContact(t *testing.T) {
	body, err := os.ReadFile("testdata/disclosure-triage-runbook.md")
	if err != nil {
		t.Fatalf("read runbook: %v", err)
	}

	if !strings.Contains(string(body), ContactURI) {
		t.Fatalf("runbook missing contact URI %q", ContactURI)
	}
	if !strings.Contains(string(body), "CRA Article 14") || !strings.Contains(string(body), "24-hour") || !strings.Contains(string(body), "ENISA") {
		t.Fatalf("runbook missing CRA Article 14 / 24-hour / ENISA marker: %q", body)
	}
}

// TestPolicyDoc_DriftsNotFromEmbed guards against an accidental edit to the
// embedded cvd-policy.md that isn't also applied to testdata/cvd-policy.md —
// a package-local golden copy, not docs/, so this stays isolated from
// unrelated docs restructuring.
//
// NOTE: this narrows what the test proves. It used to diff the served policy
// against docs/security/cvd-policy.md, the actual published doc; that file
// was deleted when docs/security/* got folded into docs/quality/security.md
// (which already reads differently in places — e.g. its Signing section vs.
// this package's cvd-policy.md). This test no longer catches that kind of
// drift against the public docs surface; it only catches the embed and this
// package-local copy going out of sync with each other.
func TestPolicyDoc_DriftsNotFromEmbed(t *testing.T) {
	canonical, err := os.ReadFile("testdata/cvd-policy.md")
	if err != nil {
		t.Fatalf("read canonical policy doc: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, SecurityPolicyPath, nil)
	w := httptest.NewRecorder()

	NewSecurityPolicyHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want %d, got %d", http.StatusOK, w.Code)
	}

	served := w.Body.Bytes()
	if !bytes.Equal(served, canonical) {
		t.Fatalf("embedded policy drifted from canonical doc")
	}
	for _, marker := range []string{"safe-harbor", "triage"} {
		if !strings.Contains(strings.ToLower(string(canonical)), marker) {
			t.Fatalf("canonical policy missing %q marker", marker)
		}
		if !strings.Contains(strings.ToLower(string(served)), marker) {
			t.Fatalf("served policy missing %q marker", marker)
		}
	}
}
