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

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
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

func orgHandlerForTest(db *sql.DB, store *crmcore.OrgStore) *OrganizationHandler {
	return NewOrganizationHandler(store, crmcore.NewRelationshipStore(db), crmcore.NewDealStore(db), crmcore.NewActivityStore(db), &crmapprovals.DBVerifier{DB: db})
}

func TestOrganizationHandler_List_EmptyWorkspace(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	h := orgHandlerForTest(db, crmcore.NewOrgStore(db))
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
	person, err := personStore.Create(ctx, personSeed, nil)
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

	h := orgHandlerForTest(db, orgStore)
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

	h := orgHandlerForTest(db, crmcore.NewOrgStore(db))
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

	h := orgHandlerForTest(db, crmcore.NewOrgStore(db))
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

	h := orgHandlerForTest(db, crmcore.NewOrgStore(db))
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

func TestOrganizationHandler_Get_ArchivedStillFetchable(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)
	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "Archivable-" + ids.New(),
		Source: "test", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := orgStore.Archive(ctx, org.ID, orgHandlerTestWS); err != nil {
		t.Fatalf("archive org: %v", err)
	}

	h := orgHandlerForTest(db, orgStore)
	req := httptest.NewRequest(http.MethodGet, "/organizations/"+org.ID, nil)
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 (PO-AC-6: archived orgs stay fetchable), body=%s", w.Code, w.Body.String())
	}

	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox WHERE topic='organization.archived' AND entity_id=$1::uuid`,
		org.ID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count event_outbox: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("want 1 organization.archived outbox row, got %d", eventCount)
	}
	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='organization' AND entity_id=$1::uuid AND action='archive'`,
		org.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_log: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("want 1 audit_log archive row, got %d", auditCount)
	}
}

func TestOrganizationHandler_Update_StaleIfMatchAndMalformed(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)
	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "Updatable-" + ids.New(),
		Source: "test", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	h := orgHandlerForTest(db, orgStore)

	reqMalformed := httptest.NewRequest(http.MethodPatch, "/organizations/"+org.ID, strings.NewReader(`{"display_name":"X"}`))
	reqMalformed.Header.Set("If-Match", "not-a-number")
	reqMalformed = withOrgWorkspace(reqMalformed)
	wMalformed := httptest.NewRecorder()
	h.ServeHTTP(wMalformed, reqMalformed)
	if wMalformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed If-Match status=%d want 400, body=%s", wMalformed.Code, wMalformed.Body.String())
	}

	reqStale := httptest.NewRequest(http.MethodPatch, "/organizations/"+org.ID, strings.NewReader(`{"display_name":"Y"}`))
	reqStale.Header.Set("If-Match", "999")
	reqStale = withOrgWorkspace(reqStale)
	wStale := httptest.NewRecorder()
	h.ServeHTTP(wStale, reqStale)
	if wStale.Code != http.StatusConflict {
		t.Fatalf("stale If-Match status=%d want 409, body=%s", wStale.Code, wStale.Body.String())
	}
	var problem map[string]any
	if err := json.Unmarshal(wStale.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem["code"] != "version_skew" {
		t.Fatalf("want code=version_skew, got %v", problem)
	}

	reqOK := httptest.NewRequest(http.MethodPatch, "/organizations/"+org.ID, strings.NewReader(`{"display_name":"Z"}`))
	reqOK.Header.Set("If-Match", "1")
	reqOK = withOrgWorkspace(reqOK)
	wOK := httptest.NewRecorder()
	h.ServeHTTP(wOK, reqOK)
	if wOK.Code != http.StatusOK {
		t.Fatalf("valid If-Match status=%d want 200, body=%s", wOK.Code, wOK.Body.String())
	}

	var eventCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM event_outbox WHERE topic='organization.updated' AND entity_id=$1::uuid`,
		org.ID,
	).Scan(&eventCount); err != nil {
		t.Fatalf("count event_outbox: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("want 1 organization.updated outbox row, got %d", eventCount)
	}
}

func TestOrganizationHandler_List_ClassificationAndRelevanceFilter(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)

	classification := "partner"
	matchingName := "PartnerHigh-" + ids.New()
	if _, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: matchingName,
		Classification: &classification, Relevance: 75,
		Source: "test", CapturedBy: "human:test",
	}); err != nil {
		t.Fatalf("create matching org: %v", err)
	}
	if _, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "PartnerLow-" + ids.New(),
		Classification: &classification, Relevance: 10,
		Source: "test", CapturedBy: "human:test",
	}); err != nil {
		t.Fatalf("create low-relevance org: %v", err)
	}
	otherClass := "vendor"
	if _, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "Vendor-" + ids.New(),
		Classification: &otherClass, Relevance: 90,
		Source: "test", CapturedBy: "human:test",
	}); err != nil {
		t.Fatalf("create vendor org: %v", err)
	}

	h := orgHandlerForTest(db, orgStore)
	req := httptest.NewRequest(http.MethodGet, "/organizations?classification=partner&relevance_gte=50&sort=strength", nil)
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Data) != 1 {
		t.Fatalf("want 1 matching org, got %d: %v", len(page.Data), page.Data)
	}
	if page.Data[0]["display_name"] != matchingName || page.Data[0]["classification"] != "partner" {
		t.Fatalf("unexpected filtered row: %v", page.Data[0])
	}
}

func TestOrganizationHandler_List_DomainAndOwnerFilter(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	ownerID := ids.New()
	if _, err := db.Exec(
		`INSERT INTO app_user (id, workspace_id, email, display_name)
		 VALUES ($1::uuid, $2::uuid, $3, 'Owner')
		 ON CONFLICT (id) DO NOTHING`,
		ownerID, orgHandlerTestWS, ownerID+"@test.example",
	); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)
	owned, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "Owned-" + ids.New(),
		OwnerID: &ownerID,
		Domains: []crmcore.OrganizationDomain{{Domain: "Owned.Example"}},
		Source:  "test", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create owned org: %v", err)
	}
	if _, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS, DisplayName: "Unowned-" + ids.New(),
		Source: "test", CapturedBy: "human:test",
	}); err != nil {
		t.Fatalf("create unowned org: %v", err)
	}

	h := orgHandlerForTest(db, orgStore)

	reqDomain := httptest.NewRequest(http.MethodGet, "/organizations?domain=owned.example", nil)
	reqDomain = withOrgWorkspace(reqDomain)
	wDomain := httptest.NewRecorder()
	h.ServeHTTP(wDomain, reqDomain)
	if wDomain.Code != http.StatusOK {
		t.Fatalf("domain filter status=%d want 200, body=%s", wDomain.Code, wDomain.Body.String())
	}
	var pageDomain struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(wDomain.Body.Bytes(), &pageDomain); err != nil {
		t.Fatalf("decode domain resp: %v", err)
	}
	if len(pageDomain.Data) != 1 || pageDomain.Data[0]["id"] != owned.ID {
		t.Fatalf("domain filter: want exactly org %s, got %v", owned.ID, pageDomain.Data)
	}

	reqOwner := httptest.NewRequest(http.MethodGet, "/organizations?owner_id="+ownerID, nil)
	reqOwner = withOrgWorkspace(reqOwner)
	wOwner := httptest.NewRecorder()
	h.ServeHTTP(wOwner, reqOwner)
	if wOwner.Code != http.StatusOK {
		t.Fatalf("owner filter status=%d want 200, body=%s", wOwner.Code, wOwner.Body.String())
	}
	var pageOwner struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(wOwner.Body.Bytes(), &pageOwner); err != nil {
		t.Fatalf("decode owner resp: %v", err)
	}
	if len(pageOwner.Data) != 1 || pageOwner.Data[0]["id"] != owned.ID {
		t.Fatalf("owner_id filter: want exactly org %s, got %v", owned.ID, pageOwner.Data)
	}
}

func TestOrganizationHandler_FullLifecycle_ListFiltersAcrossEndpoints(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)

	ownerID := ids.New()
	if _, err := db.Exec(
		`INSERT INTO app_user (id, workspace_id, email, display_name)
		 VALUES ($1::uuid, $2::uuid, $3, 'Lifecycle Owner')
		 ON CONFLICT (id) DO NOTHING`,
		ownerID, orgHandlerTestWS, ownerID+"@test.example",
	); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}

	h := orgHandlerForTest(db, crmcore.NewOrgStore(db))
	postOrg := func(body string) map[string]any {
		req := httptest.NewRequest(http.MethodPost, "/organizations", strings.NewReader(body))
		req = withOrgWorkspace(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create status=%d want 201, body=%s", w.Code, w.Body.String())
		}
		var created map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode created: %v", err)
		}
		return created
	}

	targetClassification := "partner-" + ids.New()
	targetDomain := "Acme-" + ids.New() + ".COM"
	targetDomainLower := strings.ToLower(targetDomain)
	target := postOrg(`{
		"display_name": "Acme Inc",
		"classification": "` + targetClassification + `",
		"relevance": 75,
		"owner_id": "` + ownerID + `",
		"domains": [{"domain": "` + targetDomain + `", "is_primary": true}],
		"source": "test",
		"captured_by": "human:test"
	}`)
	postOrg(`{
		"display_name": "Control Co",
		"classification": "vendor",
		"relevance": 10,
		"source": "test",
		"captured_by": "human:test"
	}`)

	if domains, ok := target["domains"].([]any); !ok || len(domains) != 1 {
		t.Fatalf("create did not normalize domains: %v", target["domains"])
	} else if dom, ok := domains[0].(map[string]any); !ok || dom["domain"] != targetDomainLower {
		t.Fatalf("create did not normalize domains: %v", target["domains"])
	}
	targetID, _ := target["id"].(string)

	listOne := func(rawURL string) []map[string]any {
		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		req = withOrgWorkspace(req)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list %s status=%d want 200, body=%s", rawURL, w.Code, w.Body.String())
		}
		var page struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode list %s: %v", rawURL, err)
		}
		return page.Data
	}

	if rows := listOne("/organizations?classification=" + targetClassification + "&relevance_gte=50"); len(rows) != 1 || rows[0]["id"] != targetID {
		t.Fatalf("classification/relevance filter mismatch: %v", rows)
	}
	if rows := listOne("/organizations?domain=" + targetDomainLower); len(rows) != 1 || rows[0]["id"] != targetID {
		t.Fatalf("domain filter mismatch: %v", rows)
	}
	if rows := listOne("/organizations?owner_id=" + ownerID); len(rows) != 1 || rows[0]["id"] != targetID {
		t.Fatalf("owner_id filter mismatch: %v", rows)
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/organizations/"+targetID, nil)
	reqGet = withOrgWorkspace(reqGet)
	wGet := httptest.NewRecorder()
	h.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("get status=%d want 200, body=%s", wGet.Code, wGet.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(wGet.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if got["id"] != targetID {
		t.Fatalf("get returned wrong org: %v", got["id"])
	}

	reqPatch := httptest.NewRequest(http.MethodPatch, "/organizations/"+targetID, strings.NewReader(`{"display_name":"Acme Prime"}`))
	reqPatch.Header.Set("If-Match", "1")
	reqPatch = withOrgWorkspace(reqPatch)
	wPatch := httptest.NewRecorder()
	h.ServeHTTP(wPatch, reqPatch)
	if wPatch.Code != http.StatusOK {
		t.Fatalf("patch status=%d want 200, body=%s", wPatch.Code, wPatch.Body.String())
	}
	var patched map[string]any
	if err := json.Unmarshal(wPatch.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if patched["display_name"] != "Acme Prime" {
		t.Fatalf("patch did not update display_name: %v", patched["display_name"])
	}

	reqDelete := httptest.NewRequest(http.MethodDelete, "/organizations/"+targetID, nil)
	reqDelete = withOrgWorkspace(reqDelete)
	wDelete := httptest.NewRecorder()
	h.ServeHTTP(wDelete, reqDelete)
	if wDelete.Code != http.StatusOK {
		t.Fatalf("delete status=%d want 200, body=%s", wDelete.Code, wDelete.Body.String())
	}

	reqArchived := httptest.NewRequest(http.MethodGet, "/organizations/"+targetID, nil)
	reqArchived = withOrgWorkspace(reqArchived)
	wArchived := httptest.NewRecorder()
	h.ServeHTTP(wArchived, reqArchived)
	if wArchived.Code != http.StatusOK {
		t.Fatalf("archived get status=%d want 200, body=%s", wArchived.Code, wArchived.Body.String())
	}
	var archived map[string]any
	if err := json.Unmarshal(wArchived.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archived: %v", err)
	}
	if archived["archived_at"] == nil {
		t.Fatalf("expected archived_at on archived org, got %v", archived)
	}

	if rows := listOne("/organizations?domain=" + targetDomainLower); len(rows) != 0 {
		t.Fatalf("archived org leaked through domain filter: %v", rows)
	}
}
