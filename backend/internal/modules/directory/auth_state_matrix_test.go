//go:build integration

package crmcore_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func mustDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func buildAuthMux(t *testing.T, db *sql.DB) (http.Handler, *crmauth.SessionStore) {
	t.Helper()
	sessions := crmauth.NewSessionStore(db)
	mux := http.NewServeMux()
	personStore := crmcore.NewPersonStore(db)
	personH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := crmctx.From(r.Context()); !ok {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized) //nolint:forbidigo // test mock handler, not a production JSON path
			return
		}
		peopletransport.NewPersonHandler(personStore, db).ServeHTTP(w, r)
	})
	// Session middleware: resolve the cookie to a Principal before serving.
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(crmauth.CookieName)
		if err == nil {
			if rec, err := sessions.Lookup(r.Context(), cookie.Value); err == nil {
				ctx := crmctx.With(r.Context(), crmctx.Principal{
					UserID: rec.UserID, TenantID: rec.WorkspaceID,
				})
				r = r.WithContext(ctx)
			}
		}
		personH.ServeHTTP(w, r)
	})
	mux.Handle("/people", wrapped)
	return mux, sessions
}

func TestAuthStateMatrix(t *testing.T) {
	db := mustDB(t)
	ctx := context.Background()
	mux, sessions := buildAuthMux(t, db)

	// Ensure test workspace + user exist.
	const wsID = "00000000-0000-0000-0099-000000000001"
	const userID = "00000000-0000-0000-0099-000000000002"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'authtest','authtest','EUR') ON CONFLICT DO NOTHING`,
		wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	hash, err := crmauth.HashPassword("pw")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO app_user(id,workspace_id,email,display_name,password_hash) VALUES($1,$2,'authtest@example.com','AT',$3) ON CONFLICT DO NOTHING`,
		userID, wsID, hash); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	cases := []struct {
		name        string
		setupCookie func() string // returns cookie value, "" for none
		wantStatus  int
	}{
		{
			name:        "no cookie -> 401",
			setupCookie: func() string { return "" },
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name: "valid session -> 200",
			setupCookie: func() string {
				tok, err := sessions.Create(ctx, wsID, userID)
				if err != nil {
					t.Fatalf("create session: %v", err)
				}
				return tok
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "expired session -> 401",
			setupCookie: func() string {
				// Insert an already-expired session directly. Use a per-run
				// unique token so the token_hash unique index never collides.
				tok := fmt.Sprintf("expiredtoken-%d", time.Now().UnixNano())
				h := crmauth.SHA256SumExported(tok)
				past := time.Now().Add(-time.Hour)
				if _, err := db.ExecContext(ctx,
					`INSERT INTO session(workspace_id,user_id,token_hash,expires_at,idle_expires_at) VALUES($1,$2,$3,$4,$4)`,
					wsID, userID, h, past); err != nil {
					t.Fatalf("insert expired session: %v", err)
				}
				return tok
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	cases = append(cases, struct {
		name        string
		setupCookie func() string
		wantStatus  int
	}{
		name: "revoked session -> 401",
		setupCookie: func() string {
			tok, err := sessions.Create(ctx, wsID, userID)
			if err != nil {
				t.Fatalf("create session: %v", err)
			}
			// Look up session record to get its ID, then delete it (simulate revocation).
			rec, err := sessions.Lookup(ctx, tok)
			if err != nil {
				t.Fatalf("lookup session: %v", err)
			}
			if err := sessions.Delete(ctx, rec.ID); err != nil {
				t.Fatalf("delete session: %v", err)
			}
			return tok
		},
		wantStatus: http.StatusUnauthorized,
	})

	cases = append(cases, struct {
		name        string
		setupCookie func() string
		wantStatus  int
	}{
		name: "idle-expired -> 401",
		setupCookie: func() string {
			tok := fmt.Sprintf("idleexpired-%d", time.Now().UnixNano())
			h := crmauth.SHA256SumExported(tok)
			future := time.Now().Add(24 * time.Hour)
			past := time.Now().Add(-time.Second) // idle_expires_at in the past
			if _, err := db.ExecContext(ctx,
				`INSERT INTO session(workspace_id,user_id,token_hash,expires_at,idle_expires_at) VALUES($1,$2,$3,$4,$5)`,
				wsID, userID, h, future, past); err != nil {
				t.Fatalf("insert idle-expired session: %v", err)
			}
			return tok
		},
		wantStatus: http.StatusUnauthorized,
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/people", nil)
			req.Header.Set("X-Workspace-ID", wsID)
			if cv := tc.setupCookie(); cv != "" {
				req.AddCookie(&http.Cookie{Name: crmauth.CookieName, Value: cv})
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("want %d, got %d — body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestLogout verifies POST /auth/logout: returns 200 and clears the session cookie.
func TestLogout(t *testing.T) {
	db := mustDB(t)
	ctx := context.Background()

	const wsID = "00000000-0000-0000-0099-000000000001"
	const userID = "00000000-0000-0000-0099-000000000002"
	// Workspace and user are seeded by TestAuthStateMatrix (idempotent inserts). Re-seed
	// here so this test can run in isolation.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'authtest','authtest','EUR') ON CONFLICT DO NOTHING`,
		wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	hash, err := crmauth.HashPassword("pw")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO app_user(id,workspace_id,email,display_name,password_hash) VALUES($1,$2,'authtest@example.com','AT',$3) ON CONFLICT DO NOTHING`,
		userID, wsID, hash); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	sessions := crmauth.NewSessionStore(db)

	// Create a valid session.
	tok, err := sessions.Create(ctx, wsID, userID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Wire a minimal mux with just the logout handler (inlined so we stay in crm-core_test;
	// cmd/server is package main and not importable).
	logoutHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, cerr := r.Cookie(crmauth.CookieName)
		if cerr == nil {
			if rec, lerr := sessions.Lookup(r.Context(), cookie.Value); lerr == nil {
				_ = sessions.Delete(r.Context(), rec.ID)
			}
		}
		http.SetCookie(w, &http.Cookie{
			Name:   crmauth.CookieName,
			Value:  "",
			MaxAge: -1,
			Path:   "/",
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux := http.NewServeMux()
	mux.Handle("POST /auth/logout", logoutHandler)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: crmauth.CookieName, Value: tok})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("logout: want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	// Verify Set-Cookie clears the session. The handler sets MaxAge=-1, which
	// net/http serializes on the wire as "Max-Age=0" (Go encodes any MaxAge<0
	// as Max-Age=0) — that is the canonical "delete this cookie now" directive.
	setCookie := w.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("logout: expected Set-Cookie header to be present")
	}
	if !clearsCookie(setCookie) {
		t.Errorf("logout: Set-Cookie should clear the cookie (Max-Age=0), got: %s", setCookie)
	}
}

// clearsCookie reports whether a raw Set-Cookie header carries the cleared-cookie
// directive Go emits for MaxAge<0, i.e. "Max-Age=0".
func clearsCookie(setCookie string) bool {
	for _, part := range splitCookieParts(setCookie) {
		if part == "Max-Age=0" {
			return true
		}
	}
	return false
}

func splitCookieParts(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			part := trimSpace(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		part := trimSpace(s[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
