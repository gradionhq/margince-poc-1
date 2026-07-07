//go:build integration

package crosscutting_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	peopletransport "github.com/gradionhq/margince/backend/internal/modules/people/transport"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
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
	personH := wrap(peopletransport.NewPersonHandler(
		people.NewPersonStore(db),
		relationships.NewRelationshipStore(db),
		deals.NewDealStore(db),
		activities.NewActivityStore(db),
		&crmapprovals.DBVerifier{DB: db},
	))
	mux.Handle("/people", personH)
	mux.Handle("/people/", personH)
	return mux
}

func TestHTTPConformance_PeopleList(t *testing.T) {
	db := sqlDB(t)

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

	if w.Code != http.StatusOK {
		t.Fatalf("GET /people: want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := resp["data"]
	if !ok {
		t.Error("response missing 'data' field")
		return
	}
	if _, isArray := data.([]interface{}); !isArray {
		t.Errorf("'data' field must be a JSON array, got %T", data)
	}
}
