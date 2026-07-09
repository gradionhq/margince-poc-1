//go:build integration

// handler_org_composite_test.go — ported from modules/directory/transport/handler_org_composite_test.go
// (package transport; crmcore imports replaced with per-module stores;
// handler constructor updated for approvalsport.Verifier parameter).
package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	actAdapters "github.com/gradionhq/margince/backend/internal/modules/activities/adapters"
	actDomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	people "github.com/gradionhq/margince/backend/internal/modules/people"
	relAdapters "github.com/gradionhq/margince/backend/internal/modules/relationships/adapters"
	relDomain "github.com/gradionhq/margince/backend/internal/modules/relationships/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const orgCompositeWS = "00000000-0000-0000-0000-000000000043"

func withOrgWorkspaceID(r *http.Request, wsID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: wsID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestOrganizationHandler_Get_Composite360(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, orgCompositeWS)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, orgCompositeWS, "org-360-ws", "org-360-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgCompositeWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := orgAdapters.NewOrgStore(db)
	relStore := relAdapters.NewRelationshipStore(db)
	dealStore := deals.NewDealStore(db)
	actStore := actAdapters.NewActivityStore(db)
	h := orgHandlerForTest(db, orgStore)

	org, err := orgStore.Create(ctx, orgDomain.Organization{WorkspaceID: orgCompositeWS, DisplayName: "Composite Org", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	person, err := people.NewPersonStore(db).Create(ctx, people.Person{WorkspaceID: orgCompositeWS, FullName: "Employee", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	if _, err := relStore.Create(ctx, relDomain.Relationship{WorkspaceID: orgCompositeWS, Kind: "employment", PersonID: &person.ID, OrganizationID: &org.ID, Source: p0.Source, CapturedBy: p0.CapturedBy}); err != nil {
		t.Fatalf("seed relationship: %v", err)
	}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: orgCompositeWS, Name: "Composite Pipeline"})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: orgCompositeWS, PipelineID: pl.ID, Name: "Open", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	d := deals.NewDeal("Composite Deal", pl.ID, st.ID, p0)
	d.WorkspaceID = orgCompositeWS
	d.OrganizationID = &org.ID
	createdDeal, err := dealStore.Create(ctx, d, "", nil)
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	act, _, err := actStore.Create(ctx, actDomain.Activity{WorkspaceID: orgCompositeWS, Kind: "call", OccurredAt: time.Now(), Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO activity_link (workspace_id, activity_id, entity_type, organization_id) VALUES ($1,$2,'organization',$3)`, orgCompositeWS, act.ID, org.ID); err != nil {
		t.Fatalf("link activity: %v", err)
	}

	otherOrg, err := orgStore.Create(ctx, orgDomain.Organization{WorkspaceID: orgCompositeWS, DisplayName: "Other Org", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	otherPerson, err := people.NewPersonStore(db).Create(ctx, people.Person{WorkspaceID: orgCompositeWS, FullName: "Not This Org's Employee", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed other person: %v", err)
	}
	if _, err := relStore.Create(ctx, relDomain.Relationship{WorkspaceID: orgCompositeWS, Kind: "employment", PersonID: &otherPerson.ID, OrganizationID: &otherOrg.ID, Source: p0.Source, CapturedBy: p0.CapturedBy}); err != nil {
		t.Fatalf("seed unrelated relationship: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/organizations/"+org.ID, nil)
	req = withOrgWorkspaceID(req, orgCompositeWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /organizations/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got orgDomain.Organization
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Relationships) != 1 || got.Relationships[0].PersonID == nil || *got.Relationships[0].PersonID != person.ID {
		t.Fatalf("relationships = %+v; want exactly one, for person %s", got.Relationships, person.ID)
	}
	if len(got.Deals) != 1 || got.Deals[0].ID != createdDeal.ID {
		t.Fatalf("deals = %+v; want exactly one, id %s", got.Deals, createdDeal.ID)
	}
	if len(got.Activities) != 1 || got.Activities[0].ID != act.ID {
		t.Fatalf("activities = %+v; want exactly one, id %s", got.Activities, act.ID)
	}
}

// TestOrganizationHandler_Get_EmptyCompositeShowsEmptyArrays_NotNull guards
// against a regression where relationships/deals/activities marshaled as
// JSON `null` (or vanished from the body entirely) for an organization with
// zero linked rows, because Organization's own `omitempty` composite tags
// drop a zero-length slice regardless of nil-vs-empty. The get() response
// must always show `[]`.
func TestOrganizationHandler_Get_EmptyCompositeShowsEmptyArrays_NotNull(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, orgCompositeWS)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, orgCompositeWS, "org-360-ws", "org-360-empty-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgCompositeWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := orgAdapters.NewOrgStore(db)
	h := orgHandlerForTest(db, orgStore)

	org, err := orgStore.Create(ctx, orgDomain.Organization{WorkspaceID: orgCompositeWS, DisplayName: "Lonely Org", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/organizations/"+org.ID, nil)
	req = withOrgWorkspaceID(req, orgCompositeWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /organizations/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()

	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"relationships", "deals", "activities"} {
		v, present := got[key]
		if !present {
			t.Fatalf("key %q absent from body entirely; want present as []: %s", key, body)
		}
		if v == nil {
			t.Fatalf("key %q is JSON null; want []: %s", key, body)
		}
		arr, ok := v.([]any)
		if !ok || len(arr) != 0 {
			t.Fatalf("key %q = %v (%T); want empty array []", key, v, v)
		}
	}
	for _, needle := range []string{`"relationships":[]`, `"deals":[]`, `"activities":[]`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("body does not contain literal %q: %s", needle, body)
		}
	}
}

func TestOrganizationHandler_Get_ArchivedStillFetchableWithComposite(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, orgCompositeWS)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgCompositeWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := orgAdapters.NewOrgStore(db)
	h := orgHandlerForTest(db, orgStore)

	org, err := orgStore.Create(ctx, orgDomain.Organization{WorkspaceID: orgCompositeWS, DisplayName: "Archive Me Org", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := orgStore.Archive(ctx, org.ID, orgCompositeWS); err != nil {
		t.Fatalf("archive: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/organizations/"+org.ID, nil)
	req = withOrgWorkspaceID(req, orgCompositeWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("archived org GET: want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrganizationHandler_Get_NonexistentID_Returns404(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, orgCompositeWS)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, orgCompositeWS, "org-360-ws", "org-360-404-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	h := orgHandlerForTest(db, orgAdapters.NewOrgStore(db))

	req := httptest.NewRequest(http.MethodGet, "/organizations/00000000-0000-0000-0000-0000000000ff", nil)
	req = withOrgWorkspaceID(req, orgCompositeWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /organizations/{nonexistent-id}: want 404, got %d: %s", w.Code, w.Body.String())
	}
	var problem map[string]any
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem["code"] != "not_found" {
		t.Fatalf("code = %v, want not_found", problem["code"])
	}
}

const orgCompositeOtherWS = "00000000-0000-0000-0000-000000000048"

func TestOrganizationHandler_Get_ForeignWorkspaceID_Returns404(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, orgCompositeWS)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, orgCompositeWS, "org-360-ws", "org-360-fw-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, orgCompositeOtherWS, "org-360-other-ws", "org-360-other-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed other workspace: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgCompositeWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := orgAdapters.NewOrgStore(db)
	h := orgHandlerForTest(db, orgStore)

	org, err := orgStore.Create(ctx, orgDomain.Organization{WorkspaceID: orgCompositeWS, DisplayName: "Tenant A Org", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	// Switch RLS to the other workspace and request the same id with a
	// principal scoped to that other workspace — the row belongs to
	// orgCompositeWS, so it must 404, never leak.
	orgHandlerSetRLS(t, db, orgCompositeOtherWS)
	req := httptest.NewRequest(http.MethodGet, "/organizations/"+org.ID, nil)
	req = withOrgWorkspaceID(req, orgCompositeOtherWS)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /organizations/{id} from foreign workspace: want 404, got %d: %s", w.Code, w.Body.String())
	}
	var problem map[string]any
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatal(err)
	}
	if problem["code"] != "not_found" {
		t.Fatalf("code = %v, want not_found", problem["code"])
	}
}

func TestOrganizationHandler_Get_Composite360_PerfBudgetAndPaginationCap(t *testing.T) {
	db := openDealTestDB(t)
	const ws = "00000000-0000-0000-0000-000000000044"
	orgHandlerSetRLS(t, db, ws)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, ws, "org-360-perf-ws", "org-360-perf-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: ws, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	orgStore := orgAdapters.NewOrgStore(db)
	relStore := relAdapters.NewRelationshipStore(db)
	h := orgHandlerForTest(db, orgStore)

	org, err := orgStore.Create(ctx, orgDomain.Organization{WorkspaceID: ws, DisplayName: "Perf Org", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	personStore := people.NewPersonStore(db)
	for i := 0; i < 60; i++ {
		person, err := personStore.Create(ctx, people.Person{WorkspaceID: ws, FullName: "Perf Employee", Source: p0.Source, CapturedBy: p0.CapturedBy}, nil)
		if err != nil {
			t.Fatalf("seed person %d: %v", i, err)
		}
		if _, err := relStore.Create(ctx, relDomain.Relationship{WorkspaceID: ws, Kind: "employment", PersonID: &person.ID, OrganizationID: &org.ID, Source: p0.Source, CapturedBy: p0.CapturedBy}); err != nil {
			t.Fatalf("seed relationship %d: %v", i, err)
		}
	}

	const iterations = 30
	durations := make([]time.Duration, 0, iterations)
	var lastRelCount int
	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest(http.MethodGet, "/organizations/"+org.ID, nil)
		req = withOrgWorkspaceID(req, ws)
		w := httptest.NewRecorder()
		start := time.Now()
		h.ServeHTTP(w, req)
		elapsed := time.Since(start)
		if w.Code != http.StatusOK {
			t.Fatalf("iteration %d: status=%d body=%s", i, w.Code, w.Body.String())
		}
		var got orgDomain.Organization
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode iteration %d: %v", i, err)
		}
		lastRelCount = len(got.Relationships)
		durations = append(durations, elapsed)
	}
	if lastRelCount != 50 {
		t.Fatalf("relationships array len = %d; want capped at 50", lastRelCount)
	}
	p95 := percentileDuration(durations, 95)
	t.Logf("org-360 GET p95 over %d iterations: %v", iterations, p95)
	if p95 > 150*time.Millisecond {
		t.Errorf("p95 %v exceeds PERF-2's 150ms budget", p95)
	}
}

func percentileDuration(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(d))
	copy(sorted, d)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
