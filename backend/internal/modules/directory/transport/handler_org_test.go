//go:build integration

package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const orgHandlerTestWS = "00000000-0000-0000-0000-000000000030"

func orgHandlerSetRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`SELECT set_config('app.workspace_id', $1, false)`, wsID); err != nil {
		t.Fatal("orgHandlerSetRLS:", err)
	}
}

func seedOrgHandlerWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	orgHandlerSetRLS(t, db, orgHandlerTestWS)
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		orgHandlerTestWS,
		"org-handler-ws",
		"org-handler-ws-"+time.Now().Format("20060102150405"),
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func withOrgWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestOrganizationHandler_List_EmptyWorkspace(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	h := NewOrganizationHandler(crmcore.NewOrgStore(db))
	req := httptest.NewRequest(http.MethodGet, "/organizations", nil)
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, ok := resp["data"]
	if !ok {
		t.Fatal("missing 'data' key")
	}
	items := data.([]any)
	if len(items) != 0 {
		t.Fatalf("want empty data:[], got %d items", len(items))
	}
}

func TestOrganizationHandler_List_WithAggregates(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}
	orgStore := crmcore.NewOrgStore(db)

	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "TestCo-" + ids.New(),
		Source: "test", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	personStore := crmcore.NewPersonStore(db)
	personSeed := crmcore.NewPerson("Agent-"+ids.New(), p0)
	personSeed.WorkspaceID = orgHandlerTestWS
	person, err := personStore.Create(ctx, personSeed)
	if err != nil {
		t.Fatalf("create person: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO relationship(workspace_id, kind, person_id, organization_id, role, source, captured_by)
		 VALUES ($1::uuid, 'employment', $2::uuid, $3::uuid, NULL, 'test', 'human:test')`,
		orgHandlerTestWS, person.ID, org.ID,
	); err != nil {
		t.Fatalf("seed employment: %v", err)
	}

	h := NewOrganizationHandler(orgStore)
	req := httptest.NewRequest(http.MethodGet, "/organizations", nil)
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data := resp["data"].([]any)
	var found bool
	for _, item := range data {
		m := item.(map[string]any)
		if m["id"] == org.ID {
			found = true
			if cc, _ := m["contact_count"].(float64); cc != 1 {
				t.Errorf("want contact_count=1, got %v", m["contact_count"])
			}
		}
	}
	if !found {
		t.Fatalf("org %s not found in response", org.ID)
	}
}

func TestOrganizationHandler_List_InvalidSort(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	h := NewOrganizationHandler(crmcore.NewOrgStore(db))
	req := httptest.NewRequest(http.MethodGet, "/organizations?sort=bogus", nil)
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422", w.Code)
	}
}
