//go:build integration

package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

const dealGetTestWS = "00000000-0000-0000-0000-000000000045"

func withDealGetWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: dealGetTestWS, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestDealHandler_Get_Composite360(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, dealGetTestWS)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, dealGetTestWS, "deal-360-ws", "deal-360-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealGetTestWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: dealGetTestWS, Name: "Deal360 Pipeline"})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: dealGetTestWS, PipelineID: pl.ID, Name: "Open", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}

	dealStore := crmcore.NewDealStore(db)
	relStore := crmcore.NewRelationshipStore(db)
	activityStore := crmcore.NewActivityStore(db)
	h := NewDealHandler(dealStore, relStore, activityStore, db)

	d := crmcore.NewDeal("Deal360 Test Deal", pl.ID, st.ID, p0)
	d.WorkspaceID = dealGetTestWS
	created, err := dealStore.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	person, err := crmcore.NewPersonStore(db).Create(ctx, crmcore.Person{WorkspaceID: dealGetTestWS, FullName: "Stakeholder", Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	if _, err := relStore.Create(ctx, crmcore.Relationship{WorkspaceID: dealGetTestWS, Kind: "deal_stakeholder", DealID: &created.ID, PersonID: &person.ID, Role: strPtrGet("champion"), Source: p0.Source, CapturedBy: p0.CapturedBy}); err != nil {
		t.Fatalf("seed stakeholder: %v", err)
	}
	act, err := activityStore.Create(ctx, crmcore.Activity{WorkspaceID: dealGetTestWS, Kind: "call", OccurredAt: time.Now(), Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO activity_link (workspace_id, activity_id, entity_type, deal_id) VALUES ($1,$2,'deal',$3)`, dealGetTestWS, act.ID, created.ID); err != nil {
		t.Fatalf("link activity: %v", err)
	}

	other := crmcore.NewDeal("Other Deal", pl.ID, st.ID, p0)
	other.WorkspaceID = dealGetTestWS
	otherCreated, err := dealStore.Create(ctx, other, "")
	if err != nil {
		t.Fatalf("seed other deal: %v", err)
	}
	otherPerson, err := crmcore.NewPersonStore(db).Create(ctx, crmcore.Person{WorkspaceID: dealGetTestWS, FullName: "Other Stakeholder", Source: p0.Source, CapturedBy: p0.CapturedBy})
	if err != nil {
		t.Fatalf("seed other person: %v", err)
	}
	if _, err := relStore.Create(ctx, crmcore.Relationship{WorkspaceID: dealGetTestWS, Kind: "deal_stakeholder", DealID: &otherCreated.ID, PersonID: &otherPerson.ID, Role: strPtrGet("champion"), Source: p0.Source, CapturedBy: p0.CapturedBy}); err != nil {
		t.Fatalf("seed unrelated stakeholder: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/deals/"+created.ID, nil)
	req = withDealGetWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /deals/{id}: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != created.ID {
		t.Fatalf("id = %v; want %s (flat DealDetail shape, not wrapped)", resp["id"], created.ID)
	}
	stakeholders, _ := resp["stakeholders"].([]any)
	if len(stakeholders) != 1 {
		t.Fatalf("stakeholders = %+v; want exactly one (unrelated deal's stakeholder must not leak)", stakeholders)
	}
	timeline, _ := resp["timeline"].([]any)
	if len(timeline) != 1 {
		t.Fatalf("timeline = %+v; want exactly one", timeline)
	}
}

func TestDealHandler_Get_ArchivedStillFetchable(t *testing.T) {
	db := openDealTestDB(t)
	orgHandlerSetRLS(t, db, dealGetTestWS)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealGetTestWS, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: dealGetTestWS, Name: "Archive Pipeline"})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: dealGetTestWS, PipelineID: pl.ID, Name: "Open", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}

	dealStore := crmcore.NewDealStore(db)
	h := NewDealHandler(dealStore, crmcore.NewRelationshipStore(db), crmcore.NewActivityStore(db), db)

	d := crmcore.NewDeal("Archive Me Deal", pl.ID, st.ID, p0)
	d.WorkspaceID = dealGetTestWS
	created, err := dealStore.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := dealStore.Archive(ctx, created.ID, dealGetTestWS); err != nil {
		t.Fatalf("archive: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/deals/"+created.ID, nil)
	req = withDealGetWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("archived deal GET: want 200 (still fetchable by id via GetAny), got %d: %s", w.Code, w.Body.String())
	}
}

func TestDealHandler_Get_Composite360_P95Under100ms(t *testing.T) {
	db := openDealTestDB(t)
	const ws = "00000000-0000-0000-0000-000000000046"
	orgHandlerSetRLS(t, db, ws)
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,$2,$3,'EUR') ON CONFLICT (id) DO NOTHING`, ws, "deal-360-perf-ws", "deal-360-perf-"+time.Now().Format("20060102150405")); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: ws, UserID: "human:test"})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: ws, Name: "Perf Pipeline"})
	if err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{WorkspaceID: ws, PipelineID: pl.ID, Name: "Open", Position: 1, Semantic: "open", WinProbability: 50})
	if err != nil {
		t.Fatalf("seed stage: %v", err)
	}

	dealStore := crmcore.NewDealStore(db)
	h := NewDealHandler(dealStore, crmcore.NewRelationshipStore(db), crmcore.NewActivityStore(db), db)

	d := crmcore.NewDeal("Perf Deal", pl.ID, st.ID, p0)
	d.WorkspaceID = ws
	created, err := dealStore.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("seed deal: %v", err)
	}

	const iterations = 30
	durations := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest(http.MethodGet, "/deals/"+created.ID, nil)
		req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: ws, UserID: "human:test"}))
		w := httptest.NewRecorder()
		start := time.Now()
		h.ServeHTTP(w, req)
		elapsed := time.Since(start)
		if w.Code != http.StatusOK {
			t.Fatalf("iteration %d: status=%d", i, w.Code)
		}
		durations = append(durations, elapsed)
	}
	p95 := percentileDealGet(durations, 95)
	t.Logf("deal-360 GET p95 over %d iterations: %v", iterations, p95)
	if p95 > 100*time.Millisecond {
		t.Errorf("p95 %v exceeds PERF-1's 100ms budget", p95)
	}
}

func percentileDealGet(d []time.Duration, p int) time.Duration {
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

func strPtrGet(s string) *string { return &s }
