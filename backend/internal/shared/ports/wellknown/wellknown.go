// Package wellknown serves the public disclosure routes that stay outside the domain layers.
package wellknown

import (
	_ "embed"
	"net/http"
	"strings"
)

// Public disclosure route paths and RFC 9116 security.txt metadata.
const (
	ContactURI         = "mailto:security@margince.example"
	ExpiresRFC3339     = "2030-12-31T23:59:59Z"
	PolicyURL          = SecurityPolicyPath
	SecurityTxtPath    = "/.well-known/security.txt"
	SecurityPolicyPath = "/security-policy"
)

//go:embed cvd-policy.md
var cvdPolicyDoc []byte

// SecurityTxtHandler serves the RFC 9116 disclosure metadata.
type SecurityTxtHandler struct{}

// NewSecurityTxtHandler returns a SecurityTxtHandler.
func NewSecurityTxtHandler() *SecurityTxtHandler { return &SecurityTxtHandler{} }

func (h *SecurityTxtHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != SecurityTxtPath {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = strings.NewReader(
		"Contact: " + ContactURI + "\n" +
			"Expires: " + ExpiresRFC3339 + "\n" +
			"Policy: " + PolicyURL + "\n" +
			"# signature out of scope\n",
	).WriteTo(w)
}

// SecurityPolicyHandler serves the public CVD policy document.
type SecurityPolicyHandler struct{}

// NewSecurityPolicyHandler returns a SecurityPolicyHandler.
func NewSecurityPolicyHandler() *SecurityPolicyHandler { return &SecurityPolicyHandler{} }

func (h *SecurityPolicyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != SecurityPolicyPath {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(cvdPolicyDoc)
}
