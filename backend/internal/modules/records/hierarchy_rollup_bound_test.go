//go:build integration

package records_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// -- new seed helpers (distinct names; reuses seedPipelineStage/seedOpenDeal from
//    pipeline_rollup_bound_test.go in this same package) -------------------------------------

// seedOrgWithParent inserts an organization with the given (nullable) parent and owner, returning
// its id. Powers the multi-level parent-tree fixtures.
func seedOrgWithParent(t *testing.T, db *sql.DB, ws, name string, parentID, ownerID *string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(
		`INSERT INTO organization (workspace_id, name, parent_org_id, owner_id, source, captured_by)
		 VALUES ($1,$2,$3,$4,'api','human:t') RETURNING id`,
		ws, name, parentID, ownerID,
	).Scan(&id); err != nil {
		t.Fatalf("seed org %s: %v", name, err)
	}
	return id
}

// seedWonDeal inserts a 'won'-status deal (closed_at + fx_rate_to_base set, satisfying the
// deal_closed_* CHECKs) so its amount_minor_base GENERATED column is populated for the closed-won
// measure.
func seedWonDeal(t *testing.T, db *sql.DB, ws, pipelineID, stageID, orgID string, amountMinor int64, fxRate string, closedAt time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, organization_id, amount_minor,
		                    currency, fx_rate_to_base, status, closed_at, source, captured_by)
		 VALUES ($1,'RDT04Won',$2,$3,$4,$5,'USD',$6,'won',$7,'api','human:t')`,
		ws, pipelineID, stageID, orgID, amountMinor, fxRate, closedAt,
	); err != nil {
		t.Fatalf("seed won deal: %v", err)
	}
}

// seedActivity inserts an activity occurring at occurredAt and links it to orgID
// (activity_link.entity_type='organization'), the shape the 30-day activity measure counts.
func seedActivity(t *testing.T, db *sql.DB, ws, orgID string, occurredAt time.Time) {
	t.Helper()
	var actID string
	if err := db.QueryRow(
		`INSERT INTO activity (workspace_id, kind, subject, occurred_at, source, captured_by)
		 VALUES ($1,'note','RDT04',$2,'api','human:t') RETURNING id`,
		ws, occurredAt,
	).Scan(&actID); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO activity_link (workspace_id, activity_id, entity_type, organization_id)
		 VALUES ($1,$2,'organization',$3)`,
		ws, actID, orgID,
	); err != nil {
		t.Fatalf("seed activity_link: %v", err)
	}
}

// seedFXRate inserts one fx_rate row (from->to on rateDate) backing the weighted-pipeline
// AsOfFXRate lookup.
func seedFXRate(t *testing.T, db *sql.DB, ws, from, to, rate string, rateDate time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		 VALUES ($1,$2,$3,$4,$5)`,
		ws, from, to, rate, rateDate,
	); err != nil {
		t.Fatalf("seed fx_rate %s->%s: %v", from, to, err)
	}
}

// seedUserWithRowScope creates an app_user and a role granting organization.read at the given
// row_scope, assigned to that user; returns the user id (the roll-up viewer).
func seedUserWithRowScope(t *testing.T, db *sql.DB, ws, rowScope string) string {
	t.Helper()
	var userID string
	if err := db.QueryRow(
		`INSERT INTO app_user (workspace_id, email, display_name)
		 VALUES ($1,$2,'Viewer') RETURNING id`,
		ws, "viewer-"+pgtest.Uniq()+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("seed app_user: %v", err)
	}
	var roleID string
	perms := fmt.Sprintf(`{"organization":{"read":{"row_scope":%q}}}`, rowScope)
	if err := db.QueryRow(
		`INSERT INTO role (workspace_id, key, is_system, permissions)
		 VALUES ($1,$2,false,$3::jsonb) RETURNING id`,
		ws, "rdt04-"+pgtest.Uniq(), perms,
	).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO role_assignment (workspace_id, role_id, user_id) VALUES ($1,$2,$3)`,
		ws, roleID, userID,
	); err != nil {
		t.Fatalf("seed role_assignment: %v", err)
	}
	return userID
}

// seedRecordGrant grants the given user a live read record_grant on orgID.
func seedRecordGrant(t *testing.T, db *sql.DB, ws, orgID, userID string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO record_grant (workspace_id, record_type, record_id, subject_type, subject_id, access, granted_by)
		 VALUES ($1,'organization',$2,'user',$3,'read',$3)`,
		ws, orgID, userID,
	); err != nil {
		t.Fatalf("seed record_grant: %v", err)
	}
}

// setStageProbability sets a nonzero win_probability so weighted-pipeline math is observable
// (seedPipelineStage's stage defaults win_probability to 0).
func setStageProbability(t *testing.T, db *sql.DB, stageID string, prob int) {
	t.Helper()
	if _, err := db.Exec(`UPDATE stage SET win_probability=$1 WHERE id=$2`, prob, stageID); err != nil {
		t.Fatalf("set stage probability: %v", err)
	}
}

// quarterStartUTC replicates currentQuarterBounds' calendar math in UTC (the test workspace's
// default timezone) so the test can place a deal just inside / just outside the window without
// reaching into the package-private helper.
func quarterStartUTC(now time.Time) time.Time {
	n := now.UTC()
	m := ((int(n.Month())-1)/3)*3 + 1
	return time.Date(n.Year(), time.Month(m), 1, 0, 0, 0, 0, time.UTC)
}

// rollupFixture bundles the standard per-test database + workspace + pipeline/stage + FX + viewer
// setup shared by TestHierarchyRollup_FormulaAndScopes and TestHierarchyRollup_PO_AC_28_Bound.
type rollupFixture struct {
	db         *sql.DB
	ws         string
	ctx        context.Context
	pipelineID string
	stageID    string
	today      time.Time
	viewer     string
	store      *records.RollupStore
}

// newRollupFixture opens a fresh test DB/workspace, seeds a pipeline+stage at winProbability, an
// fxFrom->fxTo fx_rate row dated today, and a viewer with the given row_scope.
func newRollupFixture(t *testing.T, winProbability int, fxFrom, fxTo, fxRate string, rowScope string) rollupFixture {
	t.Helper()
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := context.Background()

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, winProbability)
	today := time.Now().UTC()
	seedFXRate(t, db, ws, fxFrom, fxTo, fxRate, today)
	viewer := seedUserWithRowScope(t, db, ws, rowScope)
	store := records.NewRollupStore(db)

	return rollupFixture{
		db:         db,
		ws:         ws,
		ctx:        ctx,
		pipelineID: pipelineID,
		stageID:    stageID,
		today:      today,
		viewer:     viewer,
		store:      store,
	}
}

// -- tests --------------------------------------------------------------------------------------

// TestHierarchyRollup_FormulaAndScopes proves RD-FORM-1: the tree-scope total equals the sum of
// every node's own self-scope figures across all three measures; scope=self returns only the
// root's own figures; and a node with no contributing rows (child B) contributes a real 0 while
// still counting toward aggregated_account_count.
func TestHierarchyRollup_FormulaAndScopes(t *testing.T) {
	rf := newRollupFixture(t, 50, "USD", "EUR", "2.0000000000", "all") // base_currency is EUR
	db, ws, ctx, pipelineID, stageID, today, viewer, store := rf.db, rf.ws, rf.ctx, rf.pipelineID, rf.stageID, rf.today, rf.viewer, rf.store

	// 3-level tree: root -> childA -> grandchild, plus sibling childB (no data at all).
	root := seedOrgWithParent(t, db, ws, "root", nil, nil)
	childA := seedOrgWithParent(t, db, ws, "childA", &root, nil)
	grandchild := seedOrgWithParent(t, db, ws, "grandchild", &childA, nil)
	childB := seedOrgWithParent(t, db, ws, "childB", &root, nil)

	usd := func(v int64) *int64 { return &v }
	fx := "2.0000000000"
	inQuarter := quarterStartUTC(today).Add(24 * time.Hour)

	// root: open 100000 USD (base 200000, weighted 100000); won base 60000; 2 recent activities.
	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, usd(100000), &fx)
	seedWonDeal(t, db, ws, pipelineID, stageID, root, 40000, "1.5000000000", inQuarter)
	seedActivity(t, db, ws, root, today.Add(-1*24*time.Hour))
	seedActivity(t, db, ws, root, today.Add(-2*24*time.Hour))

	// childA: open 50000 USD (base 100000, weighted 50000); won base 20000; 1 recent activity.
	seedOpenDeal(t, db, ws, pipelineID, stageID, &childA, usd(50000), &fx)
	seedWonDeal(t, db, ws, pipelineID, stageID, childA, 20000, "1.0000000000", inQuarter)
	seedActivity(t, db, ws, childA, today.Add(-3*24*time.Hour))

	// grandchild: open 30000 USD (base 60000, weighted 30000); 1 recent + 1 stale (>30d) activity.
	seedOpenDeal(t, db, ws, pipelineID, stageID, &grandchild, usd(30000), &fx)
	seedActivity(t, db, ws, grandchild, today.Add(-5*24*time.Hour))
	seedActivity(t, db, ws, grandchild, today.Add(-40*24*time.Hour))

	tree, err := store.Compute(ctx, root, ws, viewer, "tree")
	if err != nil {
		t.Fatalf("tree compute: %v", err)
	}
	if tree.WeightedPipelineMinor != 180000 {
		t.Errorf("tree weighted_pipeline = %d, want 180000", tree.WeightedPipelineMinor)
	}
	if tree.ClosedWonMinor != 80000 {
		t.Errorf("tree closed_won = %d, want 80000", tree.ClosedWonMinor)
	}
	if tree.ActivityCount30d != 4 {
		t.Errorf("tree activity_count_30d = %d, want 4 (the 40-day-old one is excluded)", tree.ActivityCount30d)
	}
	if tree.AggregatedAccountCount != 4 {
		t.Errorf("tree aggregated_account_count = %d, want 4", tree.AggregatedAccountCount)
	}
	if len(tree.RestrictedExcluded) != 0 {
		t.Errorf("tree restricted_excluded = %+v, want empty", tree.RestrictedExcluded)
	}
	if tree.BaseCurrency != "EUR" {
		t.Errorf("base_currency = %q, want EUR", tree.BaseCurrency)
	}
	if tree.Scope != "tree" {
		t.Errorf("scope = %q, want tree", tree.Scope)
	}

	// RD-FORM-1 reconciliation: tree total == sum of every node's own self figures.
	var sumW, sumC int64
	var sumA int
	for _, node := range []string{root, childA, grandchild, childB} {
		self, err := store.Compute(ctx, node, ws, viewer, "self")
		if err != nil {
			t.Fatalf("self compute %s: %v", node, err)
		}
		if self.Scope != "self" || self.AggregatedAccountCount != 1 || len(self.RestrictedExcluded) != 0 {
			t.Errorf("self %s: scope=%q count=%d restricted=%+v, want self/1/[]",
				node, self.Scope, self.AggregatedAccountCount, self.RestrictedExcluded)
		}
		sumW += self.WeightedPipelineMinor
		sumC += self.ClosedWonMinor
		sumA += self.ActivityCount30d
	}
	if sumW != tree.WeightedPipelineMinor || sumC != tree.ClosedWonMinor || sumA != tree.ActivityCount30d {
		t.Errorf("RD-FORM-1 broken: Σ self = (w=%d,c=%d,a=%d), tree = (w=%d,c=%d,a=%d)",
			sumW, sumC, sumA, tree.WeightedPipelineMinor, tree.ClosedWonMinor, tree.ActivityCount30d)
	}

	// Child B (no data) is a real zero-everything node, present in the count, never omitted.
	selfB, err := store.Compute(ctx, childB, ws, viewer, "self")
	if err != nil {
		t.Fatalf("self childB: %v", err)
	}
	if selfB.WeightedPipelineMinor != 0 || selfB.ClosedWonMinor != 0 || selfB.ActivityCount30d != 0 {
		t.Errorf("childB self = (w=%d,c=%d,a=%d), want all 0", selfB.WeightedPipelineMinor, selfB.ClosedWonMinor, selfB.ActivityCount30d)
	}
	if selfB.AggregatedAccountCount != 1 {
		t.Errorf("childB aggregated_account_count = %d, want 1", selfB.AggregatedAccountCount)
	}
}

// TestHierarchyRollup_RestrictedNodeAndGrant proves RBAC-honest exclusion (RD-AC-1 / RD-AC-8): a
// row_scope=own viewer sees an unreadable child in restricted_excluded (and NOT that child's own
// descendant, which is never visited), with its figures excluded from every total; a record_grant
// on that child then flips it — and its now-reachable subtree — into the totals.
func TestHierarchyRollup_RestrictedNodeAndGrant(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := context.Background()

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, 100)
	today := time.Now().UTC()
	seedFXRate(t, db, ws, "USD", "EUR", "1.0000000000", today)
	viewer := seedUserWithRowScope(t, db, ws, "own")
	other := seedUserWithRowScope(t, db, ws, "own") // a different owner
	store := records.NewRollupStore(db)

	usd := func(v int64) *int64 { return &v }
	fx := "1.0000000000"

	// root(owner=viewer) -> childA(owner=other) -> grandchild(owner=viewer); childB(owner=viewer).
	root := seedOrgWithParent(t, db, ws, "root", nil, &viewer)
	childA := seedOrgWithParent(t, db, ws, "childA", &root, &other)
	grandchild := seedOrgWithParent(t, db, ws, "grandchild", &childA, &viewer)
	childB := seedOrgWithParent(t, db, ws, "childB", &root, &viewer)

	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, usd(10000), &fx)      // weighted 10000
	seedOpenDeal(t, db, ws, pipelineID, stageID, &childA, usd(40000), &fx)    // weighted 40000 (restricted)
	seedOpenDeal(t, db, ws, pipelineID, stageID, &grandchild, usd(5000), &fx) // weighted 5000 (under restricted)
	seedOpenDeal(t, db, ws, pipelineID, stageID, &childB, usd(1000), &fx)     // weighted 1000

	// Before the grant: childA is unreadable -> excluded + disclosed; grandchild never visited.
	before, err := store.Compute(ctx, root, ws, viewer, "tree")
	if err != nil {
		t.Fatalf("before-grant compute: %v", err)
	}
	if before.WeightedPipelineMinor != 11000 {
		t.Errorf("before weighted = %d, want 11000 (root+childB only)", before.WeightedPipelineMinor)
	}
	if before.AggregatedAccountCount != 2 {
		t.Errorf("before aggregated_account_count = %d, want 2 (root, childB)", before.AggregatedAccountCount)
	}
	if len(before.RestrictedExcluded) != 1 || before.RestrictedExcluded[0].ID != childA {
		t.Fatalf("before restricted_excluded = %+v, want exactly [childA=%s]", before.RestrictedExcluded, childA)
	}
	if before.RestrictedExcluded[0].DisplayName != "childA" {
		t.Errorf("restricted display_name = %q, want childA", before.RestrictedExcluded[0].DisplayName)
	}
	for _, rn := range before.RestrictedExcluded {
		if rn.ID == grandchild {
			t.Errorf("grandchild must NOT be separately disclosed (RD-AC-8 decomposition boundary): %+v", before.RestrictedExcluded)
		}
	}

	// Grant the viewer read on childA: it — and its now-reachable readable grandchild — join the totals.
	seedRecordGrant(t, db, ws, childA, viewer)
	after, err := store.Compute(ctx, root, ws, viewer, "tree")
	if err != nil {
		t.Fatalf("after-grant compute: %v", err)
	}
	if after.WeightedPipelineMinor != 56000 {
		t.Errorf("after weighted = %d, want 56000 (root+childB+childA+grandchild)", after.WeightedPipelineMinor)
	}
	if after.AggregatedAccountCount != 4 {
		t.Errorf("after aggregated_account_count = %d, want 4", after.AggregatedAccountCount)
	}
	if len(after.RestrictedExcluded) != 0 {
		t.Errorf("after restricted_excluded = %+v, want empty", after.RestrictedExcluded)
	}
}

// TestHierarchyRollup_FXRateUnavailable proves a missing stored FX rate for a needed pair fails
// the whole read with an unwrapped *deals.FXRateUnavailableError (never a rate-of-1 fallback).
func TestHierarchyRollup_FXRateUnavailable(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := context.Background()

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, 50)
	viewer := seedUserWithRowScope(t, db, ws, "all")
	store := records.NewRollupStore(db)

	root := seedOrgWithParent(t, db, ws, "root", nil, nil)
	// A USD open deal with amount but NO fx_rate row seeded for USD->EUR.
	amt := int64(100000)
	fx := "1.0000000000"
	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, &amt, &fx)

	_, err := store.Compute(ctx, root, ws, viewer, "tree")
	var fxErr *deals.FXRateUnavailableError
	if !errors.As(err, &fxErr) {
		t.Fatalf("Compute err = %v, want *deals.FXRateUnavailableError", err)
	}
	if fxErr.Currency != "USD" {
		t.Errorf("fx error currency = %q, want USD", fxErr.Currency)
	}
}

// TestHierarchyRollup_ClosedWonQuarterBoundary proves closed_won counts only won deals whose
// closed_at falls inside the current calendar quarter.
func TestHierarchyRollup_ClosedWonQuarterBoundary(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	ctx := context.Background()

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	viewer := seedUserWithRowScope(t, db, ws, "all")
	store := records.NewRollupStore(db)

	root := seedOrgWithParent(t, db, ws, "root", nil, nil)
	qStart := quarterStartUTC(time.Now().UTC())
	inside := qStart.Add(1 * time.Hour)
	outside := qStart.Add(-1 * time.Hour)

	seedWonDeal(t, db, ws, pipelineID, stageID, root, 70000, "1.0000000000", inside)  // counts
	seedWonDeal(t, db, ws, pipelineID, stageID, root, 99000, "1.0000000000", outside) // excluded

	got, err := store.Compute(ctx, root, ws, viewer, "tree")
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if got.ClosedWonMinor != 70000 {
		t.Errorf("closed_won = %d, want 70000 (only the in-quarter deal)", got.ClosedWonMinor)
	}
}

// TestHierarchyRollup_NotFound proves a nonexistent root id is ErrNotFound.
func TestHierarchyRollup_NotFound(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)
	viewer := seedUserWithRowScope(t, db, ws, "all")
	store := records.NewRollupStore(db)

	_, err := store.Compute(context.Background(), "00000000-0000-0000-0000-000000000000", ws, viewer, "tree")
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("Compute of nonexistent root err = %v, want errs.ErrNotFound", err)
	}
}

// TestHierarchyRollup_PO_AC_28_Bound is the PO-AC-28 CI budget guard: a 200-node tree must roll up
// with p95 wall-clock < 200ms against the existing idx_org_parent index. A normal TestXxx (not a
// testing.B) so a regression actually fails CI.
func TestHierarchyRollup_PO_AC_28_Bound(t *testing.T) {
	rf := newRollupFixture(t, 50, "USD", "EUR", "1.2500000000", "all")
	db, ws, ctx, pipelineID, stageID, today, viewer, store := rf.db, rf.ws, rf.ctx, rf.pipelineID, rf.stageID, rf.today, rf.viewer, rf.store

	// 200-node tree: BFS build, branching factor 5 (root + ~4 levels, well under the CTE depth cap).
	const wantNodes = 200
	root := seedOrgWithParent(t, db, ws, "n-root", nil, nil)
	all := []string{root}
	queue := []string{root}
	for len(all) < wantNodes {
		parent := queue[0]
		queue = queue[1:]
		p := parent
		for c := 0; c < 5 && len(all) < wantNodes; c++ {
			id := seedOrgWithParent(t, db, ws, fmt.Sprintf("n-%d", len(all)), &p, nil)
			all = append(all, id)
			queue = append(queue, id)
		}
	}
	if len(all) != wantNodes {
		t.Fatalf("seeded %d nodes, want %d", len(all), wantNodes)
	}

	usd := func(v int64) *int64 { return &v }
	fx := "1.2500000000"
	inQuarter := quarterStartUTC(today).Add(24 * time.Hour)
	for _, id := range all {
		seedOpenDeal(t, db, ws, pipelineID, stageID, &id, usd(10000), &fx)
		seedWonDeal(t, db, ws, pipelineID, stageID, id, 5000, "1.0000000000", inQuarter)
		seedActivity(t, db, ws, id, today.Add(-1*24*time.Hour))
	}

	// Warm-up (prime the plan/cache), then sample.
	if _, err := store.Compute(ctx, root, ws, viewer, "tree"); err != nil {
		t.Fatalf("warm-up compute: %v", err)
	}

	const samples = 50
	durs := make([]float64, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		res, err := store.Compute(ctx, root, ws, viewer, "tree")
		durs[i] = time.Since(start).Seconds() * 1000 // ms
		if err != nil {
			t.Fatalf("sample %d compute: %v", i, err)
		}
		if res.AggregatedAccountCount != len(all) {
			t.Fatalf("sample %d aggregated_account_count = %d, want %d", i, res.AggregatedAccountCount, len(all))
		}
	}

	sort.Float64s(durs)
	p95 := durs[int(math.Ceil(0.95*float64(samples)))-1] // ceil(0.95*N)-1
	t.Logf("PO-AC-28: %d-node tree roll-up p95 = %.1fms over %d samples", len(all), p95, samples)
	if p95 >= 200 {
		t.Errorf("p95 roll-up latency = %.1fms, want < 200ms (PO-AC-28 budget)", p95)
	}
}
