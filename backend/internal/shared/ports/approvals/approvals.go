// Package approvalsport is the Tier-0 approval-token verify/consume seam
// (WS-D-b, AC-D2, D9): platform/toolgate depends on this interface at the
// tool boundary — never on the modules/approvals domain implementation
// directly, which would recreate the exact platform-module→domain-module edge
// AC-E6 removes elsewhere. modules/approvals keeps the real *sql.DB-backed
// VerifyAndConsume and satisfies Verifier via a thin adapter (DBVerifier),
// constructed once at cmd/api composition and injected into every
// toolgate.Enforce call site.
package approvalsport

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

// Binding is the exact operation an approval token authorizes — the
// verifier's claims must match every field here (APPR-AC-7's claim-binding
// requirement).
type Binding struct {
	WorkspaceID   string
	Tool          string
	DiffHash      string
	TargetVersion *int64
}

// Verifier validates a single-use X-Approval-Token against the Binding it
// must authorize, then atomically consumes it (replay returns an error).
type Verifier interface {
	VerifyAndConsume(ctx context.Context, token string, want Binding) error
}

// HashDiff computes the diff_hash claim binding: a deterministic hash of the
// canonical fields a token authorizes. encoding/json sorts map[string]any
// keys alphabetically, so this is stable regardless of insertion order. Both
// the minting path and every toolgate.Enforce call must use this exact
// function so a token signed against a diff matches what VerifyAndConsume
// recomputes against the live request.
func HashDiff(fields map[string]any) string {
	canon, _ := json.Marshal(fields)
	sum := sha256.Sum256(canon)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
