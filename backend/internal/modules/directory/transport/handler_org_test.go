//go:build integration

package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestOrganizationHandler_Create_NormalizesDomainAndWritesAuditEvent(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	h := NewOrganizationHandler(crmcore.NewOrgStore(db))
	body := `{
		"display_name": "Acme Inc",
		"domains": [{"domain": "Acme.COM", "is_primary": true}],
		"source": "test",
		"captured_by": "human:test"
	}`
	req := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader(body))
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d want 201, body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	domains, ok := created["domains"].([]any)
	if !ok || len(domains) != 1 {
		t.Fatalf("want 1 domain, got %v", created["domains"])
	}
	d0 := domains[0].(map[string]any)
	if d0["domain"] != "acme.com" {
		t.Fatalf("want domains[0].domain=acme.com, got %v", d0["domain"])
	}

	orgID, _ := created["id"].(string)

	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='organization' AND entity_id=$1::uuid AND action='create'`,
		orgID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("want 1 audit_log create row, got %d", auditCount)
	}

	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox WHERE topic='organization.created' AND entity_id=$1::uuid`,
		orgID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count event_outbox: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("want 1 organization.created outbox row, got %d", eventCount)
	}
}

func TestOrganizationHandler_Create_DuplicateDomainReturns409(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	h := NewOrganizationHandler(crmcore.NewOrgStore(db))
	first := `{
		"display_name": "First Co",
		"domains": [{"domain": "dupe.com", "is_primary": true}],
		"source": "test",
		"captured_by": "human:test"
	}`
	req1 := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader(first))
	req1 = withOrgWorkspace(req1)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("seed create status=%d want 201, body=%s", w1.Code, w1.Body.String())
	}
	var firstOrg map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &firstOrg); err != nil {
		t.Fatalf("decode first: %v", err)
	}
	firstID, _ := firstOrg["id"].(string)

	second := `{
		"display_name": "Second Co",
		"domains": [{"domain": "DUPE.com"}],
		"source": "test",
		"captured_by": "human:test"
	}`
	req2 := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader(second))
	req2 = withOrgWorkspace(req2)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409, body=%s", w2.Code, w2.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["code"] != "duplicate_domain" {
		t.Fatalf("want code=duplicate_domain, got %v", problem)
	}
	details, ok := problem["details"].(map[string]any)
	if !ok {
		t.Fatalf("want details in problem body, got %v", problem)
	}
	if details["existing_id"] != firstID {
		t.Fatalf("want details.existing_id=%s, got %v", firstID, details["existing_id"])
	}
	if details["field"] != "domains[0].domain" {
		t.Fatalf("want details.field=domains[0].domain, got %v", details["field"])
	}

	// No partial org/domain row left behind by the failed second request.
	var orgCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM organization WHERE workspace_id=$1::uuid AND name='Second Co'`,
		orgHandlerTestWS,
	).Scan(&orgCount); err != nil {
		t.Fatalf("count orgs: %v", err)
	}
	if orgCount != 0 {
		t.Fatalf("want 0 'Second Co' orgs after failed create, got %d", orgCount)
	}
}
