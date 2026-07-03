//go:build integration

package crmauth_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedWorkspace inserts a minimal workspace row. workspace requires slug
// (UNIQUE) and base_currency (CHECK ^[A-Z]{3}$) with no defaults
// (backend/migrations/000002_identity_tenancy.up.sql) — both are supplied here.
func seedWorkspace(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	suffix := time.Now().Format("150405.000000")
	err := db.QueryRow(`INSERT INTO workspace (name, slug, base_currency) VALUES ($1, $2, 'USD') RETURNING id`,
		"oauth-test-ws-"+suffix, "oauth-test-ws-"+suffix).Scan(&id)
	if err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return id
}

// seedUser inserts a minimal app_user row. app_user has no password_hash
// column (credentials live elsewhere) and requires display_name NOT NULL
// with no default (backend/migrations/000002_identity_tenancy.up.sql).
func seedUser(t *testing.T, db *sql.DB, workspaceID string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`INSERT INTO app_user (workspace_id, email, display_name) VALUES ($1, $2, 'OAuth Test User') RETURNING id`,
		workspaceID, "oauth-test-"+time.Now().Format("150405.000000")+"@example.com").Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func TestOAuthClientStore_RegisterAndLookup(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	store := crmauth.NewOAuthClientStore(db)

	rec, err := store.Register(context.Background(), wsID, []string{"https://client.example/callback"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if rec.ClientID == "" {
		t.Fatal("Register must return a non-empty client_id")
	}

	got, err := store.Lookup(context.Background(), rec.ClientID)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.WorkspaceID != wsID || len(got.RedirectURIs) != 1 || got.RedirectURIs[0] != "https://client.example/callback" {
		t.Fatalf("Lookup mismatch: %+v", got)
	}
}

func TestOAuthClientStore_LookupUnknown_NotFound(t *testing.T) {
	db := testDB(t)
	store := crmauth.NewOAuthClientStore(db)

	_, err := store.Lookup(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error looking up an unregistered client_id")
	}
}

func TestAuthCodeStore_IssueAndConsume_Success(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	userID := seedUser(t, db, wsID)
	clientStore := crmauth.NewOAuthClientStore(db)
	client, err := clientStore.Register(context.Background(), wsID, []string{"https://client.example/callback"})
	if err != nil {
		t.Fatalf("Register client: %v", err)
	}

	store := crmauth.NewAuthCodeStore(db)
	verifier := "test-code-verifier-1234567890123456789012345678901234567890"
	challenge := crmauth.PKCEChallengeS256(verifier)

	rawCode, err := store.Issue(context.Background(), client.ClientID, wsID, challenge,
		"https://client.example/callback", []string{"read:person"}, userID, 10*time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if rawCode == "" {
		t.Fatal("Issue should return a non-empty code")
	}

	rec, err := store.Consume(context.Background(), rawCode, verifier)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if rec.ClientID != client.ClientID || rec.WorkspaceID != wsID || rec.GrantedBy != userID {
		t.Fatalf("Consume record mismatch: %+v", rec)
	}
	if len(rec.Scopes) != 1 || rec.Scopes[0] != "read:person" {
		t.Fatalf("Consume scopes = %+v", rec.Scopes)
	}
}

func TestAuthCodeStore_Consume_WrongVerifier_Rejected(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	userID := seedUser(t, db, wsID)
	clientStore := crmauth.NewOAuthClientStore(db)
	client, _ := clientStore.Register(context.Background(), wsID, []string{"https://client.example/callback"})

	store := crmauth.NewAuthCodeStore(db)
	challenge := crmauth.PKCEChallengeS256("correct-verifier-1234567890123456789012345678901234")
	rawCode, err := store.Issue(context.Background(), client.ClientID, wsID, challenge,
		"https://client.example/callback", []string{"read:person"}, userID, 10*time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	_, err = store.Consume(context.Background(), rawCode, "wrong-verifier-000000000000000000000000000000000")
	if !errors.Is(err, crmauth.ErrInvalidGrant) {
		t.Fatalf("Consume with wrong verifier: err = %v, want ErrInvalidGrant", err)
	}
}

func TestAuthCodeStore_Consume_ReusedCode_Rejected(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	userID := seedUser(t, db, wsID)
	clientStore := crmauth.NewOAuthClientStore(db)
	client, _ := clientStore.Register(context.Background(), wsID, []string{"https://client.example/callback"})

	store := crmauth.NewAuthCodeStore(db)
	verifier := "reuse-verifier-1234567890123456789012345678901234567890"
	challenge := crmauth.PKCEChallengeS256(verifier)
	rawCode, err := store.Issue(context.Background(), client.ClientID, wsID, challenge,
		"https://client.example/callback", []string{"read:person"}, userID, 10*time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := store.Consume(context.Background(), rawCode, verifier); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	if _, err := store.Consume(context.Background(), rawCode, verifier); !errors.Is(err, crmauth.ErrInvalidGrant) {
		t.Fatalf("reused Consume: err = %v, want ErrInvalidGrant", err)
	}
}

func TestAuthCodeStore_Consume_Expired_Rejected(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	userID := seedUser(t, db, wsID)
	clientStore := crmauth.NewOAuthClientStore(db)
	client, _ := clientStore.Register(context.Background(), wsID, []string{"https://client.example/callback"})

	store := crmauth.NewAuthCodeStore(db)
	verifier := "expired-verifier-1234567890123456789012345678901234567890"
	challenge := crmauth.PKCEChallengeS256(verifier)
	// Negative TTL: already expired the instant it's issued.
	rawCode, err := store.Issue(context.Background(), client.ClientID, wsID, challenge,
		"https://client.example/callback", []string{"read:person"}, userID, -1*time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := store.Consume(context.Background(), rawCode, verifier); !errors.Is(err, crmauth.ErrInvalidGrant) {
		t.Fatalf("expired Consume: err = %v, want ErrInvalidGrant", err)
	}
}
