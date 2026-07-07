package adapters

import (
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

func TestSignToken_RoundTrips(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "test-root-secret")
	claims := domain.TokenClaims{
		JTI: "jti-1", ApprovalID: "appr-1", WorkspaceID: "00000000-0000-0000-0000-000000000001",
		Tool: "advance_deal", DiffHash: "hash-1", Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	}
	tok, err := SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	got, err := parseToken(tok)
	if err != nil {
		t.Fatalf("parseToken: %v", err)
	}
	if got.JTI != claims.JTI || got.DiffHash != claims.DiffHash {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestParseToken_TamperedSignatureRejected(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "test-root-secret")
	claims := domain.TokenClaims{
		JTI: "jti-2", WorkspaceID: "00000000-0000-0000-0000-000000000001",
		Tool: "advance_deal", DiffHash: "hash-1", Exp: time.Now().Add(5 * time.Minute),
	}
	tok, err := SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	tampered := tok[:len(tok)-2] + "xx"
	if _, err := parseToken(tampered); err == nil {
		t.Fatal("expected tampered signature to be rejected")
	} else if err != errs.ErrApprovalTokenInvalid {
		t.Fatalf("expected ErrApprovalTokenInvalid, got %v", err)
	}
}
