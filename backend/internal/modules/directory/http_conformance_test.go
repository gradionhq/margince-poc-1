//go:build integration

// TestHTTPConformance_PeopleList is the "web + server work together" gate.
// It wires the real cmd/server HTTP handler (same as main.go) against a live
// test DB and asserts GET /people returns a valid PersonListResponse.
// Run: make infra-up && make test-db-up && make test-integration
//
// package crmcore_test (external): NewPersonHandler moved to
// modules/people/transport in the 1c restructure (task-3-brief.md), and an
// internal (package crmcore) test file cannot import people/transport without
// an import cycle (people/transport imports crmcore/directory for
// Person/PersonStore) — the external test package form used by this file's
// siblings (auth_state_matrix_test.go, rbac_object_matrix_test.go) is the
// correct, cycle-free shape, and this file only ever used crmcore's exported
// API, so no other change was needed.
package crmcore_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func buildServerMux(t *testing.T, db *sql.DB) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	wrap := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wsID := r.Header.Get("X-Workspace-ID")
			userID := r.Header.Get("X-User-ID")
			if wsID != "" {
				ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: wsID, UserID: userID})
				r = r.WithContext(ctx)
			}
			h.ServeHTTP(w, r)
		})
	}
	personStore := crmcore.NewPersonStore(db)
	personH := wrap(peopletransport.NewPersonHandler(personStore, db))
	mux.Handle("/people", personH)
	mux.Handle("/people/", personH)
	return mux
}

func TestHTTPConformance_PeopleList(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Seed a workspace so the RLS GUC is valid.
	const wsID = "00000000-0000-0000-0001-000000000099"
	_, _ = db.Exec(
		`INSERT INTO workspace(id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR')
		 ON CONFLICT (id) DO NOTHING`,
		wsID, "conformance-ws", "conformance-ws",
	)

	mux := buildServerMux(t, db)

	req := httptest.NewRequest(http.MethodGet, "/people", nil)
	req.Header.Set("X-Workspace-ID", wsID)
	req.Header.Set("X-User-ID", "human:conformance-test")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// AC-3a: status 200
	if w.Code != http.StatusOK {
		t.Fatalf("GET /people: want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	// AC-3b: Content-Type is JSON
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	// AC-3c: body decodes into a valid response structure without error
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// AC-3d: data is a JSON array (may be empty — workspace was just created)
	// Verify the structure matches PersonListResponse expectations
	data, ok := resp["data"]
	if !ok {
		t.Error("response missing 'data' field")
		return
	}
	// AC-3d: data must be a JSON array (never null)
	if _, isArray := data.([]interface{}); !isArray {
		t.Errorf("'data' field must be a JSON array, got %T", data)
	}
}
