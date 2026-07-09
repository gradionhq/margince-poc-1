//go:build integration

package adapters_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	deals "github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	orgadapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgdomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// wsFilterTest is a dedicated workspace for DealListFilter integration tests.
// Chosen outside the range used by other integration tests (001–004).
const wsFilterTest = "00000000-0000-0000-0000-000000000010"

type filterTestFixtures struct {
	pipelineID string
	stageA     string
	stageB     string
	owner1     string
	owner2     string
	org1       string
	org2       string
	// deal IDs
	dealA1 string // stageA, owner1, org1, status=open,  last_activity_at=now()    (fresh, not stalled)
	dealA2 string // stageA, owner2, org2, status=won
	dealA3 string // stageA, owner1, org1, status=lost
	dealB1 string // stageB, owner1, org2, status=open,  last_activity_at=NULL, created_at=now()-70d (stalled via created_at fallback)
	dealB2 string // stageB, owner2, org1, status=open,  last_activity_at=now()-70d (stalled under 60d threshold)
	dealB3 string // stageB, owner1, org1, status=open,  last_activity_at=now()-70d, wait_until=now()+30d (suppressed, NOT stalled)
}

func seedFilterFixture(t *testing.T) filterTestFixtures {
	t.Helper()
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsFilterTest)
	pgtest.SeedWorkspace(t, db, wsFilterTest)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsFilterTest})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	// Seed two app_users for owner_id FK.
	owner1 := "f1000000-0000-0000-0000-000000000001"
	owner2 := "f1000000-0000-0000-0000-000000000002"
	for _, uid := range []string{owner1, owner2} {
		seedAppUser(t, db, uid, wsFilterTest)
	}

	// Seed two organizations for organization_id FK.
	ostore := orgadapters.NewOrgStore(db)
	o1, err := ostore.Create(ctx, orgdomain.Organization{WorkspaceID: wsFilterTest, DisplayName: "Org1 " + pgtest.Uniq(), Source: "test", CapturedBy: "human:test"})
	if err != nil {
		t.Fatal("seed org1:", err)
	}
	o2, err := ostore.Create(ctx, orgdomain.Organization{WorkspaceID: wsFilterTest, DisplayName: "Org2 " + pgtest.Uniq(), Source: "test", CapturedBy: "human:test"})
	if err != nil {
		t.Fatal("seed org2:", err)
	}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: wsFilterTest, Name: "FilterTest " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("seed pipeline:", err)
	}

	sstore := deals.NewStageStore(db)
	stA, err := sstore.Create(ctx, deals.Stage{
		WorkspaceID: wsFilterTest, PipelineID: pl.ID,
		Name: "StageA", Position: 1, Semantic: "open", WinProbability: 30,
	})
	if err != nil {
		t.Fatal("seed stageA:", err)
	}
	stB, err := sstore.Create(ctx, deals.Stage{
		WorkspaceID: wsFilterTest, PipelineID: pl.ID,
		Name: "StageB", Position: 2, Semantic: "open", WinProbability: 60,
	})
	if err != nil {
		t.Fatal("seed stageB:", err)
	}

	ds := adapters.NewDealStore(db)

	// Every deal is seeded "open"; the won/lost cases below advance via Update.
	mk := func(name, stageID, ownerID, orgID string) domain.Deal {
		d := domain.NewDeal(name+" "+pgtest.Uniq(), pl.ID, stageID, p0)
		d.WorkspaceID = wsFilterTest
		d.OwnerID = &ownerID
		d.OrganizationID = &orgID
		d.Status = "open"
		created, err := ds.Create(ctx, d, "", nil)
		if err != nil {
			t.Fatalf("create deal %s: %v", name, err)
		}
		return created
	}

	a1 := mk("A1", stA.ID, owner1, o1.ID)
	now := time.Now().UTC()
	if _, err := db.ExecContext(context.Background(),
		`UPDATE deal SET last_activity_at=$2 WHERE id=$1::uuid`, a1.ID, now); err != nil {
		t.Fatal("set A1 last_activity_at:", err)
	}

	// A2: advance to won via Update (sets closed_at automatically).
	a2 := mk("A2", stA.ID, owner2, o2.ID)
	if _, err := ds.Update(ctx, a2.ID, wsFilterTest, map[string]any{"status": "won"}, 0); err != nil {
		t.Fatal("update A2 to won:", err)
	}

	// A3: advance to lost via Update (requires lost_reason for check constraint).
	a3 := mk("A3", stA.ID, owner1, o1.ID)
	if _, err := ds.Update(ctx, a3.ID, wsFilterTest, map[string]any{"status": "lost", "lost_reason": "test"}, 0); err != nil {
		t.Fatal("update A3 to lost:", err)
	}

	// B1: open + NULL last_activity_at, created_at backdated 70d → stalled via created_at fallback (UAT-6, 60d threshold).
	b1 := mk("B1", stB.ID, owner1, o2.ID)
	if _, err := db.ExecContext(context.Background(),
		`UPDATE deal SET created_at=$2 WHERE id=$1::uuid`, b1.ID, time.Now().UTC().Add(-70*24*time.Hour)); err != nil {
		t.Fatal("set B1 created_at:", err)
	}

	// B2: open + >60d last_activity_at → stalled under DEAL-FORM-3's 60-day threshold.
	b2 := mk("B2", stB.ID, owner2, o1.ID)
	old := time.Now().UTC().Add(-70 * 24 * time.Hour)
	if _, err := db.ExecContext(context.Background(),
		`UPDATE deal SET last_activity_at=$2 WHERE id=$1::uuid`, b2.ID, old); err != nil {
		t.Fatal("set B2 last_activity_at:", err)
	}

	// B3: open + >60d idle, but wait_until 30d in the future → suppressed, NOT stalled.
	b3 := mk("B3", stB.ID, owner1, o1.ID)
	if _, err := db.ExecContext(context.Background(),
		`UPDATE deal SET last_activity_at=$2, wait_until=$3 WHERE id=$1::uuid`,
		b3.ID, time.Now().UTC().Add(-70*24*time.Hour), time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		t.Fatal("set B3 last_activity_at/wait_until:", err)
	}

	return filterTestFixtures{
		pipelineID: pl.ID,
		stageA:     stA.ID, stageB: stB.ID,
		owner1: owner1, owner2: owner2,
		org1: o1.ID, org2: o2.ID,
		dealA1: a1.ID, dealA2: a2.ID, dealA3: a3.ID,
		dealB1: b1.ID, dealB2: b2.ID, dealB3: b3.ID,
	}
}

// TestDealListFilter exercises all DealListFilter predicates against a known fixture.
func TestDealListFilter(t *testing.T) {
	fix := seedFilterFixture(t)
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsFilterTest)
	ds := adapters.NewDealStore(db)
	ctx := context.Background()

	// scope combines a filter with the fixture's pipeline so pre-existing deals
	// from earlier test runs in wsFilterTest don't pollute the assertions.
	scope := func(f domain.DealListFilter) domain.DealListFilter {
		f.PipelineID = fix.pipelineID
		return f
	}

	t.Run("stage_id_A", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{StageID: fix.stageA}))
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealA1, fix.dealA2, fix.dealA3)
	})

	t.Run("stage_id_B", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{StageID: fix.stageB}))
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealB1, fix.dealB2, fix.dealB3)
	})

	t.Run("stageA_stageB_disjoint_union_equals_pipeline", func(t *testing.T) {
		aDeals, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{StageID: fix.stageA}))
		if err != nil {
			t.Fatal(err)
		}
		bDeals, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{StageID: fix.stageB}))
		if err != nil {
			t.Fatal(err)
		}
		pDeals, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, domain.DealListFilter{PipelineID: fix.pipelineID})
		if err != nil {
			t.Fatal(err)
		}
		aSet, bSet, pSet := makeSet(aDeals), makeSet(bDeals), makeSet(pDeals)
		for id := range aSet {
			if bSet[id] {
				t.Errorf("deal %s in both stageA and stageB results", id)
			}
		}
		union := make(map[string]bool)
		for id := range aSet {
			union[id] = true
		}
		for id := range bSet {
			union[id] = true
		}
		if !equalSets(union, pSet) {
			t.Errorf("stageA ∪ stageB ≠ pipeline:\n  union:    %v\n  pipeline: %v", sortedKeys(union), sortedKeys(pSet))
		}
	})

	t.Run("pipeline_id", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, domain.DealListFilter{PipelineID: fix.pipelineID})
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealA1, fix.dealA2, fix.dealA3, fix.dealB1, fix.dealB2, fix.dealB3)
	})

	t.Run("owner_id", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{OwnerID: fix.owner1}))
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealA1, fix.dealA3, fix.dealB1, fix.dealB3)
	})

	t.Run("organization_id", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{OrganizationID: fix.org1}))
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealA1, fix.dealA3, fix.dealB2, fix.dealB3)
	})

	t.Run("projects_owner_and_organization_id", func(t *testing.T) {
		// Defect 1 regression: ListFiltered must project owner_id and
		// organization_id, not leave them nil. Scope to a single deal (A1)
		// whose seeded owner/org are known.
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{StageID: fix.stageA, Status: "open"}))
		if err != nil {
			t.Fatal(err)
		}
		var a1 *domain.Deal
		for i := range got {
			if got[i].ID == fix.dealA1 {
				a1 = &got[i]
			}
		}
		if a1 == nil {
			t.Fatalf("deal A1 %s not in result set", fix.dealA1)
		}
		if a1.OwnerID == nil {
			t.Fatalf("owner_id is nil; want %s", fix.owner1)
		}
		if *a1.OwnerID != fix.owner1 {
			t.Errorf("owner_id = %s; want %s", *a1.OwnerID, fix.owner1)
		}
		if a1.OrganizationID == nil {
			t.Fatalf("organization_id is nil; want %s", fix.org1)
		}
		if *a1.OrganizationID != fix.org1 {
			t.Errorf("organization_id = %s; want %s", *a1.OrganizationID, fix.org1)
		}
	})

	t.Run("empty_result_is_non_nil_slice", func(t *testing.T) {
		// Defect 2 regression at the store layer: a zero-match filter must
		// return a non-nil empty slice, not nil.
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{OwnerID: "f1000000-0000-0000-0000-0000000000ff"}))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("expected zero matches, got %d", len(got))
		}
		if got == nil {
			t.Error("empty result is nil slice; want non-nil []domain.Deal{}")
		}
	})

	t.Run("status_won", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{Status: "won"}))
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealA2)
	})

	t.Run("stalled_true", func(t *testing.T) {
		// B1 (open + NULL, created_at 70d) and B2 (open + >60d) are stalled under DEAL-FORM-3;
		// B3 is suppressed by wait_until, so must NOT appear; A1 is open+fresh; A2/A3 aren't open.
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{Stalled: true}))
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, fix.dealB1, fix.dealB2)
	})

	t.Run("stageA_and_status_open", func(t *testing.T) {
		got, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, scope(domain.DealListFilter{StageID: fix.stageA, Status: "open"}))
		if err != nil {
			t.Fatal(err)
		}
		// Only A1 is stageA+open; A2=won, A3=lost
		assertIDSet(t, got, fix.dealA1)
	})

	t.Run("cursor_pagination_covers_full_pipeline", func(t *testing.T) {
		collected := make(map[string]bool)
		cursor := ""
		for {
			page, next, err := ds.ListFiltered(ctx, wsFilterTest, cursor, 2, domain.DealListFilter{PipelineID: fix.pipelineID})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range page {
				if collected[d.ID] {
					t.Errorf("duplicate deal %s in paginated results", d.ID)
				}
				collected[d.ID] = true
			}
			if next == "" {
				break
			}
			cursor = next
		}
		want := map[string]bool{fix.dealA1: true, fix.dealA2: true, fix.dealA3: true, fix.dealB1: true, fix.dealB2: true, fix.dealB3: true}
		if !equalSets(collected, want) {
			t.Errorf("pagination full coverage:\n  got:  %v\n  want: %v", sortedKeys(collected), sortedKeys(want))
		}
	})

	t.Run("no_filter_equals_list", func(t *testing.T) {
		filtered, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, domain.DealListFilter{})
		if err != nil {
			t.Fatal(err)
		}
		legacy, _, err := ds.List(ctx, wsFilterTest, "", 100)
		if err != nil {
			t.Fatal(err)
		}
		if !equalSets(makeSet(filtered), makeSet(legacy)) {
			t.Errorf("ListFiltered(zero) ≠ List:\n  filtered: %v\n  legacy:   %v",
				sortedKeys(makeSet(filtered)), sortedKeys(makeSet(legacy)))
		}
	})
}

func TestDealStore_ListFiltered_ForecastCategoryPartnerOrgSort(t *testing.T) {
	fix := seedFilterFixture(t)
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsFilterTest)
	ds := adapters.NewDealStore(db)
	ctx := context.Background()

	fc := "commit"
	d1 := domain.NewDeal("Forecast A "+pgtest.Uniq(), fix.pipelineID, fix.stageA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d1.WorkspaceID = wsFilterTest
	d1.ForecastCategory = &fc
	amt1 := int64(1000)
	d1.AmountMinor = &amt1
	partnerOrg1 := fix.org1
	d1.PartnerOrgID = &partnerOrg1
	created1, err := ds.Create(ctx, d1, "", nil)
	if err != nil {
		t.Fatalf("create d1: %v", err)
	}

	d2 := domain.NewDeal("Forecast B "+pgtest.Uniq(), fix.pipelineID, fix.stageA, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d2.WorkspaceID = wsFilterTest
	amt2 := int64(2000)
	d2.AmountMinor = &amt2
	partnerOrg2 := fix.org2
	d2.PartnerOrgID = &partnerOrg2
	if _, err := ds.Create(ctx, d2, "", nil); err != nil {
		t.Fatalf("create d2: %v", err)
	}

	out, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 20, domain.DealListFilter{PipelineID: fix.pipelineID, ForecastCategory: fc})
	if err != nil {
		t.Fatalf("ListFiltered forecast_category: %v", err)
	}
	if len(out) != 1 || out[0].Name != d1.Name {
		t.Fatalf("expected exactly Forecast A, got %+v", out)
	}

	out, _, err = ds.ListFiltered(ctx, wsFilterTest, "", 20, domain.DealListFilter{PipelineID: fix.pipelineID, Sort: "amount_minor"})
	if err != nil {
		t.Fatalf("ListFiltered sort=amount_minor: %v", err)
	}
	if len(out) < 2 || out[0].AmountMinor == nil || out[1].AmountMinor == nil || *out[0].AmountMinor > *out[1].AmountMinor {
		t.Fatalf("expected ascending amount_minor order, got %+v", out)
	}

	if len(out) > 0 && out[0].StageEnteredAt == nil {
		t.Fatal("expected StageEnteredAt to be populated on list rows")
	}

	// partner_org_id filter: only the deal seeded with the matching
	// partner_org_id (d1 -> fix.org1) should come back.
	out, _, err = ds.ListFiltered(ctx, wsFilterTest, "", 20, domain.DealListFilter{PipelineID: fix.pipelineID, PartnerOrgID: fix.org1})
	if err != nil {
		t.Fatalf("ListFiltered partner_org_id: %v", err)
	}
	if len(out) != 1 || out[0].ID != created1.ID {
		t.Fatalf("expected exactly deal d1 (%s) for partner_org_id=%s, got %+v", created1.ID, fix.org1, out)
	}
	if out[0].Name != d1.Name {
		t.Fatalf("expected Forecast A, got %+v", out)
	}
}

func TestDealStalledFlag(t *testing.T) {
	fix := seedFilterFixture(t)
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsFilterTest)
	ds := adapters.NewDealStore(db)
	ctx := context.Background()

	cases := []struct {
		id   string
		want bool
	}{
		{id: fix.dealA1, want: false},
		{id: fix.dealA2, want: false},
		{id: fix.dealA3, want: false},
		{id: fix.dealB1, want: true},
		{id: fix.dealB2, want: true},
		{id: fix.dealB3, want: false},
	}

	wantSet := make(map[string]bool, len(cases))
	for _, tc := range cases {
		got, err := ds.Get(ctx, tc.id, wsFilterTest)
		if err != nil {
			t.Fatalf("get deal %s: %v", tc.id, err)
		}
		if got.Stalled != tc.want {
			t.Errorf("deal %s stalled = %v; want %v", tc.id, got.Stalled, tc.want)
		}
		if tc.want {
			wantSet[tc.id] = true
		}
	}

	// Scope to the fixture pipeline: wsFilterTest accumulates deals across runs
	// (each seedFilterFixture adds 5 fresh uniq()-named deals), so an unscoped
	// limit-100 list ordered by id eventually drops the newest deals (e.g. B2)
	// off the first page. The sibling TestDealListFilter scopes the same way.
	listed, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, domain.DealListFilter{PipelineID: fix.pipelineID})
	if err != nil {
		t.Fatal(err)
	}
	listMap := make(map[string]bool, len(listed))
	listByID := make(map[string]domain.Deal, len(listed))
	for _, d := range listed {
		listMap[d.ID] = d.Stalled
		listByID[d.ID] = d
	}

	// Regression: the list projection previously omitted last_activity_at, so
	// every list entry returned a nil LastActivityAt even when the DB column was
	// non-null. B2 was seeded with last_activity_at = now()-70d, so both the
	// single-deal Get() path and the list path must surface a non-nil value.
	b2Get, err := ds.Get(ctx, fix.dealB2, wsFilterTest)
	if err != nil {
		t.Fatalf("get deal B2: %v", err)
	}
	if b2Get.LastActivityAt == nil {
		t.Fatal("Get(B2).LastActivityAt is nil; want non-nil (seeded now()-70d)")
	}
	b2List, ok := listByID[fix.dealB2]
	if !ok {
		t.Fatalf("deal B2 %s not in list result", fix.dealB2)
	}
	if b2List.LastActivityAt == nil {
		t.Fatal("list entry for B2 has nil LastActivityAt; want non-nil (regression: list projection omitted last_activity_at)")
	}
	for _, tc := range cases {
		got, err := ds.Get(ctx, tc.id, wsFilterTest)
		if err != nil {
			t.Fatalf("get deal %s for list agreement: %v", tc.id, err)
		}
		if got.Stalled != listMap[tc.id] {
			t.Errorf("deal %s stalled disagreement: list=%v get=%v", tc.id, listMap[tc.id], got.Stalled)
		}
	}

	filtered, _, err := ds.ListFiltered(ctx, wsFilterTest, "", 100, domain.DealListFilter{PipelineID: fix.pipelineID, Stalled: true})
	if err != nil {
		t.Fatal(err)
	}
	gotSet := makeSet(filtered)
	if !equalSets(gotSet, wantSet) {
		t.Errorf("stalled filter mismatch:\n  got:  %v\n  want: %v", sortedKeys(gotSet), sortedKeys(wantSet))
	}
}

// --- helpers ---

func makeSet(deals []domain.Deal) map[string]bool {
	m := make(map[string]bool, len(deals))
	for _, d := range deals {
		m[d.ID] = true
	}
	return m
}

func equalSets(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertIDSet(t *testing.T, got []domain.Deal, wantIDs ...string) {
	t.Helper()
	want := make(map[string]bool, len(wantIDs))
	for _, id := range wantIDs {
		want[id] = true
	}
	gotSet := makeSet(got)
	if !equalSets(gotSet, want) {
		t.Errorf("ID set mismatch:\n  got:  %v\n  want: %v", sortedKeys(gotSet), sortedKeys(want))
	}
}

// wsKanbanP95 is the workspace for the 50-card Kanban p95 + EXPLAIN tests.
const wsKanbanP95 = "00000000-0000-0000-0000-000000000011"

// TestDealListFilter_P95AndExplain seeds a 50-card stage column, asserts the
// stage_id-filtered query is served by an index (not a Seq Scan), and checks
// that the p95 server-side duration is under 150 ms.
//
// EXPLAIN is run inside a tx with SET LOCAL enable_seqscan=off because on a
// tiny fixture (50 rows) the planner's default is a Seq Scan — disabling it
// forces the planner to find the cheapest available index and reveals whether
// one covers the predicate.
func TestDealListFilter_P95AndExplain(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsKanbanP95)
	pgtest.SeedWorkspace(t, db, wsKanbanP95)

	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsKanbanP95})
	p0 := prov.Provenance{Source: "test", CapturedBy: "human:test"}

	pstore := deals.NewPipelineStore(db)
	pl, err := pstore.Create(ctx, deals.Pipeline{WorkspaceID: wsKanbanP95, Name: "P95 " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("seed pipeline:", err)
	}
	sstore := deals.NewStageStore(db)
	st, err := sstore.Create(ctx, deals.Stage{
		WorkspaceID: wsKanbanP95, PipelineID: pl.ID,
		Name: "KanbanCol", Position: 1, Semantic: "open", WinProbability: 50,
	})
	if err != nil {
		t.Fatal("seed stage:", err)
	}

	ds := adapters.NewDealStore(db)
	for i := 0; i < 50; i++ {
		d := domain.NewDeal("Card "+pgtest.Uniq(), pl.ID, st.ID, p0)
		d.WorkspaceID = wsKanbanP95
		if _, err := ds.Create(ctx, d, "", nil); err != nil {
			t.Fatalf("seed deal %d: %v", i, err)
		}
	}

	t.Run("explain_no_seq_scan", func(t *testing.T) {
		// The query ListFiltered generates for a stage_id filter (no pipeline prefix, no cursor):
		// WHERE workspace_id=$1::uuid AND archived_at IS NULL AND ($2 = '' OR id::text > $2) AND stage_id=$4::uuid ORDER BY id LIMIT $3
		// We inline the UUIDs for the EXPLAIN call.
		explainSQL := `EXPLAIN SELECT id, workspace_id, name, pipeline_id, stage_id,
			amount_minor, currency, status, version, source, captured_by, created_at, updated_at
		FROM deal
		WHERE workspace_id='` + wsKanbanP95 + `'::uuid AND archived_at IS NULL
		  AND ('' = '' OR id::text > '')
		  AND stage_id='` + st.ID + `'::uuid
		ORDER BY id LIMIT 51`

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("begin tx:", err)
		}
		defer func() { _ = tx.Rollback() }()

		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			t.Fatal("disable seqscan:", err)
		}

		rows, err := tx.QueryContext(ctx, explainSQL)
		if err != nil {
			t.Fatal("EXPLAIN:", err)
		}
		defer rows.Close()

		var plan strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal("scan plan line:", err)
			}
			plan.WriteString(line)
			plan.WriteString("\n")
		}
		if err := rows.Err(); err != nil {
			t.Fatal("plan rows:", err)
		}
		t.Logf("stage_id query plan (seqscan=off):\n%s", plan.String())

		if strings.Contains(plan.String(), "Seq Scan on deal") {
			t.Fatalf("stage_id filter fell back to Seq Scan with seqscan off — index coverage required, plan:\n%s", plan.String())
		}

		// Expect any pre-existing deal index to cover the predicate.  The planner may
		// choose idx_deal_stage, idx_deal_pipeline, idx_deal_ws_live, or idx_deal_owner
		// depending on statistics; the critical invariant is that NO seq scan occurs.
		wantIndexes := []string{
			"idx_deal_stage", "idx_deal_pipeline",
			"idx_deal_ws_live", "idx_deal_owner", "idx_deal_stalled",
		}
		found := false
		for _, idx := range wantIndexes {
			if strings.Contains(plan.String(), idx) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected Index Scan on one of %v, got:\n%s", wantIndexes, plan.String())
		}
	})

	t.Run("p95_under_150ms", func(t *testing.T) {
		const iterations = 30
		durations := make([]time.Duration, 0, iterations)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, _, err := ds.ListFiltered(ctx, wsKanbanP95, "", 50, domain.DealListFilter{StageID: st.ID})
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("ListFiltered iteration %d: %v", i, err)
			}
			durations = append(durations, elapsed)
		}

		p95 := filterPercentile(durations, 95)
		t.Logf("stage_id filter p95 over %d iterations: %v", iterations, p95)
		if p95 > 150*time.Millisecond {
			t.Errorf("p95 %v exceeds 150ms budget", p95)
		}
	})
}

// filterPercentile returns the p-th percentile of a duration slice (sorted ascending).
func filterPercentile(d []time.Duration, p int) time.Duration {
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
