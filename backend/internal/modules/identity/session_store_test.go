//go:build integration

package crmauth_test

import (
	"context"
	"errors"
	"testing"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
)

// TestSessionRestoredColumns proves AC-C5/D4: SessionStore.Create persists
// user_agent/ip, Lookup returns them, and after Revoke, Lookup stops
// authenticating the session (ErrNotFound rather than a hard delete).
func TestSessionRestoredColumns(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	userID := seedUser(t, db, wsID)
	store := crmauth.NewSessionStore(db)

	rawToken, err := store.Create(context.Background(), wsID, userID, "Mozilla/5.0 test-agent", "203.0.113.7")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec, err := store.Lookup(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.UserAgent == nil || *rec.UserAgent != "Mozilla/5.0 test-agent" {
		t.Fatalf("UserAgent = %v, want %q", rec.UserAgent, "Mozilla/5.0 test-agent")
	}
	if rec.IP == nil || *rec.IP != "203.0.113.7" {
		t.Fatalf("IP = %v, want %q", rec.IP, "203.0.113.7")
	}
	if rec.RevokedAt != nil {
		t.Fatalf("RevokedAt = %v, want nil before Revoke", rec.RevokedAt)
	}

	if err := store.Revoke(context.Background(), rec.ID, wsID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := store.Lookup(context.Background(), rawToken); !errors.Is(err, crmauth.ErrNotFound) {
		t.Fatalf("Lookup after revoke: err = %v, want ErrNotFound", err)
	}
}

// TestSessionRevoke_UnknownSession_NotFound proves Revoke mirrors
// PassportStore.Revoke's not-found/already-revoked semantics.
func TestSessionRevoke_UnknownSession_NotFound(t *testing.T) {
	db := testDB(t)
	store := crmauth.NewSessionStore(db)
	err := store.Revoke(context.Background(),
		"00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, crmauth.ErrNotFound) {
		t.Fatalf("Revoke unknown session: err = %v, want ErrNotFound", err)
	}
}

// TestSessionCreate_EmptyUserAgentIP_StoresNull proves an empty caller-supplied
// user_agent/ip maps to SQL NULL, not the literal empty string.
func TestSessionCreate_EmptyUserAgentIP_StoresNull(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	userID := seedUser(t, db, wsID)
	store := crmauth.NewSessionStore(db)

	rawToken, err := store.Create(context.Background(), wsID, userID, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	rec, err := store.Lookup(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.UserAgent != nil {
		t.Fatalf("UserAgent = %v, want nil for empty input", rec.UserAgent)
	}
	if rec.IP != nil {
		t.Fatalf("IP = %v, want nil for empty input", rec.IP)
	}
}
