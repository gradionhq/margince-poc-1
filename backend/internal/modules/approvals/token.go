// Package crmapprovals implements the X-Approval-Token verify/consume seam
// (gate issue #59, Option 1): a minimal, self-contained compact-JWS
// (HMAC-SHA256) parser/validator with single-use consumption tracking.
// Minting (an approval decision issuing a token to the requesting agent) is
// explicitly out of scope for this ticket — a fast-follow ticket wires the
// real mint flow onto SignToken's shape and retrofits T06 (person/org merge)
// onto VerifyAndConsume once it lands.
package crmapprovals

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

// TokenClaims is the ApprovalToken claim set (crm.yaml
// components.schemas.ApprovalToken / APPR-WIRE-1).
type TokenClaims struct {
	JTI           string    `json:"jti"`
	ApprovalID    string    `json:"approval_id"`
	WorkspaceID   string    `json:"workspace_id"`
	PassportID    *string   `json:"passport_id"`
	OnBehalfOf    *string   `json:"on_behalf_of"`
	Tool          string    `json:"tool"`
	DiffHash      string    `json:"diff_hash"`
	TargetVersion *int64    `json:"target_version"`
	Exp           time.Time `json:"exp"`
	SingleUse     bool      `json:"single_use"`
}

type jwsHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

const tokenSigningSecretEnv = "APPROVAL_TOKEN_SIGNING_SECRET"

// signingKey derives the per-workspace HMAC key from the server-held root
// secret (APPROVAL_TOKEN_SIGNING_SECRET) as HMAC-SHA256(root, workspaceID) —
// avoids needing a per-workspace key-storage table for this minimal seam.
func signingKey(workspaceID string) ([]byte, error) {
	root := os.Getenv(tokenSigningSecretEnv)
	if root == "" {
		return nil, fmt.Errorf("crmapprovals: %s is required", tokenSigningSecretEnv)
	}
	mac := hmac.New(sha256.New, []byte(root))
	mac.Write([]byte(workspaceID))
	return mac.Sum(nil), nil
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// SignToken mints a compact JWS from claims — TEST-ONLY. Production minting
// (an approval decision issuing a token) is a separate ticket (see package
// doc); this exists purely so this ticket's tests can construct valid tokens
// against the same signingKey/verify path without a production caller.
func SignToken(claims TokenClaims) (string, error) {
	key, err := signingKey(claims.WorkspaceID)
	if err != nil {
		return "", err
	}
	header := b64url(mustJSON(jwsHeader{Alg: "HS256", Typ: "JWT"}))
	payload := b64url(mustJSON(claims))
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(signingInput))
	return signingInput + "." + b64url(mac.Sum(nil)), nil
}

// parseToken decodes and verifies a compact JWS's signature and shape,
// without touching the consumption table. Any malformed/unverifiable input
// returns errs.ErrApprovalTokenInvalid.
func parseToken(token string) (TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	var claims TokenClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	sigGiven, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	key, err := signingKey(claims.WorkspaceID)
	if err != nil {
		return TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if subtle.ConstantTimeCompare(sigGiven, mac.Sum(nil)) != 1 {
		return TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	return claims, nil
}

// Binding is now the Tier-0 approvalsport.Binding (D9, WS-D-b): the shared
// shape lives in the port so platform/toolgate never has to import this
// domain module directly.
//
// Deliberately excludes passport_id, even though crm.yaml's ApprovalToken
// schema and the X-Approval-Token parameter description both state the
// server checks it: crmctx.Principal (shared/kernel/crmctx/crmctx.go) carries
// only UserID/TenantID/IsAgent today — no Agent Seat Passport identity — so
// there is nothing on the request to bind passport_id against yet. This is a
// known, explicit gap (not a silent drop): VerifyAndConsume still parses and
// exposes claims.PassportID for a future caller, but no current caller can
// check it. Flag this explicitly in the PR description as a fast-follow once
// a Passport-bearing seam reaches this handler (ADR-0013).
type Binding = approvalsport.Binding

// VerifyAndConsume validates a compact-JWS X-Approval-Token against the
// operation it must authorize, then atomically records its jti as consumed
// (single-use, APPR-PARAM-3). Any mismatch, expiry, malformed signature, or
// replay returns errs.ErrApprovalTokenInvalid (API-ERR-11). See Binding's doc
// comment for the known passport_id-binding gap this seam does not yet close.
func VerifyAndConsume(ctx context.Context, db *sql.DB, token string, want Binding) error {
	claims, err := parseToken(token)
	if err != nil {
		return err
	}
	if claims.WorkspaceID != want.WorkspaceID || claims.Tool != want.Tool || claims.DiffHash != want.DiffHash {
		return errs.ErrApprovalTokenInvalid
	}
	if want.TargetVersion != nil && (claims.TargetVersion == nil || *claims.TargetVersion != *want.TargetVersion) {
		return errs.ErrApprovalTokenInvalid
	}
	if claims.JTI == "" || time.Now().After(claims.Exp) {
		return errs.ErrApprovalTokenInvalid
	}
	consumed, err := consumeJTI(ctx, db, claims.WorkspaceID, claims.JTI)
	if err != nil {
		return err
	}
	if !consumed {
		return errs.ErrApprovalTokenInvalid // replay
	}
	return nil
}

// consumeJTI atomically claims jti for one-time use via INSERT ... ON
// CONFLICT DO NOTHING under the workspace-scoped app role (data-model §1.3);
// returns false if it was already consumed (replay).
func consumeJTI(ctx context.Context, db *sql.DB, workspaceID, jti string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE margince_app`); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO consumed_approval_token (jti, workspace_id)
		VALUES ($1, $2::uuid)
		ON CONFLICT (jti) DO NOTHING`, jti, workspaceID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, nil
	}
	return true, tx.Commit()
}

// DBVerifier adapts the *sql.DB-backed VerifyAndConsume above to the Tier-0
// approvalsport.Verifier shape platform/toolgate depends on (D9). The
// concrete instance is constructed once at cmd/api composition and injected
// into every toolgate.Enforce call site via whichever handler already holds
// an approvalsport.Verifier-typed field.
type DBVerifier struct {
	DB *sql.DB
}

// VerifyAndConsume implements approvalsport.Verifier.
func (v *DBVerifier) VerifyAndConsume(ctx context.Context, token string, want approvalsport.Binding) error {
	return VerifyAndConsume(ctx, v.DB, token, want)
}
