package model

import "regexp"

// regexpStripper applies each pattern→replacement pair in order. It is the one
// canonical SecretStripper; crm-ai and crm-capture both build it via
// NewSecretStripper rather than maintaining divergent pattern sets.
type regexpStripper struct {
	patterns []*regexp.Regexp
	repls    []string
}

func (s *regexpStripper) Strip(payload string) string {
	for i, p := range s.patterns {
		payload = p.ReplaceAllString(payload, s.repls[i])
	}
	return payload
}

// redaction markers: bare replaces the whole match; prefixed keeps capture
// group 1 (the non-secret prefix) and redacts only the value.
const (
	redacted         = `[REDACTED]`
	redactedPrefixed = `${1}` + redacted
)

// credPatterns is the union of every credential form the prior two strippers
// recognised. Prefixed forms keep the non-secret prefix and redact only the
// value; bare forms redact the whole match.
var credPatterns = []struct {
	pat  string
	repl string
}{
	// Bearer tokens: "Authorization: Bearer <token>"
	{`(?i)(bearer\s+)\S+`, redactedPrefixed},
	// api_key=<value> / api-key=<value> / api_key="..."
	{`(?i)(api[_-]key\s*=\s*"?)[^\s"]+`, redactedPrefixed},
	// password=<value> or password: <value>
	{`(?i)(password\s*[=:]\s*)\S+`, redactedPrefixed},
	// token: <value> (key-colon form)
	{`(?i)(token\s*:\s*)\S+`, redactedPrefixed},
	// GitHub PATs (catch-all for any not caught by the context patterns above)
	{`ghp_[A-Za-z0-9]+`, redacted},
	// sk- prefix tokens: OpenAI/Anthropic style (catch-all)
	{`sk-[A-Za-z0-9_-]+`, redacted},
	// AWS access key IDs: AKIA followed by 16 base32 chars
	{`AKIA[A-Z0-9]{16}`, redacted},
}

// NewSecretStripper returns the canonical SecretStripper that redacts API keys,
// bearer tokens, GitHub PATs, AWS access keys, and passwords from a payload
// before it can egress to any model or external endpoint. It does NOT
// pseudonymize PII (names/emails pass through — A8).
//
//nolint:ireturn // seam returns the SecretStripper interface by design
func NewSecretStripper() SecretStripper {
	s := &regexpStripper{}
	for _, cp := range credPatterns {
		s.patterns = append(s.patterns, regexp.MustCompile(cp.pat))
		s.repls = append(s.repls, cp.repl)
	}
	return s
}
