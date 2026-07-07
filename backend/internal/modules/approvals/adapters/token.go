// Package adapters: token signing/verification and DB-backed approval store.
package adapters

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

	"github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

type jwsHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

const tokenSigningSecretEnv = "APPROVAL_TOKEN_SIGNING_SECRET"

// signingKey derives the per-workspace HMAC key from the server-held root secret.
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

// SignToken mints a compact JWS from claims — TEST-ONLY.
func SignToken(claims domain.TokenClaims) (string, error) {
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

// parseToken decodes and verifies a compact JWS's signature and shape.
func parseToken(token string) (domain.TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return domain.TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return domain.TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	var claims domain.TokenClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return domain.TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	sigGiven, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return domain.TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	key, err := signingKey(claims.WorkspaceID)
	if err != nil {
		return domain.TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if subtle.ConstantTimeCompare(sigGiven, mac.Sum(nil)) != 1 {
		return domain.TokenClaims{}, errs.ErrApprovalTokenInvalid
	}
	return claims, nil
}

// Binding is now the Tier-0 approvalsport.Binding (D9, WS-D-b).
type Binding = approvalsport.Binding

// VerifyAndConsume validates a compact-JWS X-Approval-Token against the
// operation it must authorize, then atomically records its jti as consumed.
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
		return errs.ErrApprovalTokenInvalid
	}
	return nil
}

// consumeJTI atomically claims jti for one-time use.
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

// DBVerifier adapts the *sql.DB-backed VerifyAndConsume to the Tier-0
// approvalsport.Verifier shape platform/toolgate depends on (D9).
type DBVerifier struct {
	DB *sql.DB
}

// VerifyAndConsume implements approvalsport.Verifier.
func (v *DBVerifier) VerifyAndConsume(ctx context.Context, token string, want approvalsport.Binding) error {
	return VerifyAndConsume(ctx, v.DB, token, want)
}
