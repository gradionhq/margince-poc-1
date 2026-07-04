//go:build integration

package crmapprovals

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func openApprovalTokenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const tokenTestWorkspaceID = "00000000-0000-0000-0000-0000000000a1"

func seedTokenTestWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency)
		VALUES ($1, 't12-token-ws', 't12-token-ws', 'EUR') ON CONFLICT (id) DO NOTHING`,
		tokenTestWorkspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func TestVerifyAndConsume_SucceedsOnceThenRejectsReplay(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "it-root-secret")
	db := openApprovalTokenTestDB(t)
	seedTokenTestWorkspace(t, db)
	ctx := context.Background()

	claims := TokenClaims{
		JTI: "it-jti-" + tokenTestWorkspaceID, ApprovalID: "appr-it-1", WorkspaceID: tokenTestWorkspaceID,
		Tool: "advance_deal", DiffHash: "hash-it-1", Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	}
	tok, err := SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	want := Binding{WorkspaceID: tokenTestWorkspaceID, Tool: "advance_deal", DiffHash: "hash-it-1"}

	if err := VerifyAndConsume(ctx, db, tok, want); err != nil {
		t.Fatalf("first VerifyAndConsume: %v", err)
	}
	if err := VerifyAndConsume(ctx, db, tok, want); err == nil {
		t.Fatal("expected replay to be rejected")
	}
}

func TestVerifyAndConsume_RejectsMismatchedBinding(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "it-root-secret")
	db := openApprovalTokenTestDB(t)
	seedTokenTestWorkspace(t, db)
	ctx := context.Background()

	claims := TokenClaims{
		JTI: "it-jti-mismatch-" + tokenTestWorkspaceID, WorkspaceID: tokenTestWorkspaceID,
		Tool: "advance_deal", DiffHash: "hash-it-2", Exp: time.Now().Add(5 * time.Minute),
	}
	tok, err := SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	// Different diff_hash than the one signed — must be rejected as mis-bound.
	wrong := Binding{WorkspaceID: tokenTestWorkspaceID, Tool: "advance_deal", DiffHash: "different-hash"}
	if err := VerifyAndConsume(ctx, db, tok, wrong); err == nil {
		t.Fatal("expected mis-bound diff_hash to be rejected")
	}
}

func TestVerifyAndConsume_RejectsExpired(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "it-root-secret")
	db := openApprovalTokenTestDB(t)
	seedTokenTestWorkspace(t, db)
	ctx := context.Background()

	claims := TokenClaims{
		JTI: "it-jti-expired-" + tokenTestWorkspaceID, WorkspaceID: tokenTestWorkspaceID,
		Tool: "advance_deal", DiffHash: "hash-it-3", Exp: time.Now().Add(-1 * time.Minute),
	}
	tok, err := SignToken(claims)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	want := Binding{WorkspaceID: tokenTestWorkspaceID, Tool: "advance_deal", DiffHash: "hash-it-3"}
	if err := VerifyAndConsume(ctx, db, tok, want); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
