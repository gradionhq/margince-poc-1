//go:build integration

package records_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgtransport "github.com/gradionhq/margince/backend/internal/modules/organizations/transport"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	crmauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// rollupMoneyWire mirrors hierarchyRollupMoney for JSON decoding (transport types are unexported).
type rollupMoneyWire struct {
	AmountMinor *int64  `json:"amount_minor"`
	Currency    *string `json:"currency"`
}

// rollupWire mirrors hierarchyRollupResponse for JSON decoding.
type rollupWire struct {
	RootID                 string          `json:"root_id"`
	Scope                  string          `json:"scope"`
	WeightedPipeline       rollupMoneyWire `json:"weighted_pipeline"`
	ClosedWon              rollupMoneyWire `json:"closed_won"`
	ActivityCount30d       int             `json:"activity_count_30d"`
	AggregatedAccountCount int             `json:"aggregated_account_count"`
	RestrictedExcluded     []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"restricted_excluded"`
}

// newRollupHTTPHandler builds an OrganizationHandler with real stores (including
// records.RollupStore) for end-to-end HTTP integration testing.
func newRollupHTTPHandler(db *sql.DB) http.Handler {
	return orgtransport.NewOrganizationHandler(
		orgAdapters.NewOrgStore(db),
		relationships.NewRelationshipStore(db),
		deals.NewDealStore(db),
		activities.NewActivityStore(db),
		records.NewRollupStore(db),
		&crmapprovals.DBVerifier{DB: db},
	)
}

// doRollupGet sends GET /organizations/{orgID}/hierarchy-rollup to h with the given
// workspace+user principal, optionally with ?scope={scope} (empty string = omit param).
func doRollupGet(t *testing.T, h http.Handler, wsID, userID, orgID, scope string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/organizations/" + orgID + "/hierarchy-rollup"
	if scope != "" {
		path += "?scope=" + scope
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: wsID, UserID: userID}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeRollupHTTP JSON-decodes a 200 OK hierarchy-rollup response body.
func decodeRollupHTTP(t *testing.T, rec *httptest.ResponseRecorder) rollupWire {
	t.Helper()
	var resp rollupWire
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode rollup body: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

// seedHTTPUserNoOrgPerm seeds a user whose role has deal.read but NO organization
// permissions — used to trigger the 403 path of RbacMiddleware.
func seedHTTPUserNoOrgPerm(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var userID string
	if err := db.QueryRow(
		`INSERT INTO app_user (workspace_id, email, display_name) VALUES ($1,$2,'NoOrgPerm') RETURNING id`,
		ws, "noperm-"+pgtest.Uniq()+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("seedHTTPUserNoOrgPerm app_user: %v", err)
	}
	var roleID string
	if err := db.QueryRow(
		`INSERT INTO role (workspace_id, key, is_system, permissions) VALUES ($1,$2,false,'{"deal":{"read":{"row_scope":"all"}}}'::jsonb) RETURNING id`,
		ws, "noperm-"+pgtest.Uniq(),
	).Scan(&roleID); err != nil {
		t.Fatalf("seedHTTPUserNoOrgPerm role: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO role_assignment (workspace_id, role_id, user_id) VALUES ($1,$2,$3)`,
		ws, roleID, userID,
	); err != nil {
		t.Fatalf("seedHTTPUserNoOrgPerm role_assignment: %v", err)
	}
	return userID
}

// TestHierarchyRollupHTTP_TreeAndSelfScope proves that scope=tree (default) includes
// the full subtree and scope=self returns only the root node's own figures.
func TestHierarchyRollupHTTP_TreeAndSelfScope(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, 100)
	today := time.Now().UTC()
	seedFXRate(t, db, ws, "USD", "EUR", "1.0000000000", today)
	viewer := seedUserWithRowScope(t, db, ws, "all")

	root := seedOrgWithParent(t, db, ws, "root", nil, nil)
	child := seedOrgWithParent(t, db, ws, "child", &root, nil)

	usdAmt := func(v int64) *int64 { v2 := v; return &v2 }
	fx := "1.0000000000"
	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, usdAmt(10000), &fx)
	seedActivity(t, db, ws, root, today.Add(-1*24*time.Hour))
	seedOpenDeal(t, db, ws, pipelineID, stageID, &child, usdAmt(20000), &fx)
	seedActivity(t, db, ws, child, today.Add(-2*24*time.Hour))

	h := newRollupHTTPHandler(db)

	// omitted scope defaults to "tree"
	recDefault := doRollupGet(t, h, ws, viewer, root, "")
	if recDefault.Code != http.StatusOK {
		t.Fatalf("default scope: status=%d body=%s", recDefault.Code, recDefault.Body)
	}
	defResp := decodeRollupHTTP(t, recDefault)
	if defResp.Scope != "tree" {
		t.Errorf("default scope body.scope=%q, want tree", defResp.Scope)
	}
	if defResp.RootID != root {
		t.Errorf("root_id=%s, want %s", defResp.RootID, root)
	}
	if defResp.AggregatedAccountCount != 2 {
		t.Errorf("tree aggregated_account_count=%d, want 2", defResp.AggregatedAccountCount)
	}
	if defResp.WeightedPipeline.AmountMinor == nil || *defResp.WeightedPipeline.AmountMinor != 30000 {
		t.Errorf("tree weighted_pipeline.amount_minor=%v, want 30000", defResp.WeightedPipeline.AmountMinor)
	}
	if defResp.ActivityCount30d != 2 {
		t.Errorf("tree activity_count_30d=%d, want 2", defResp.ActivityCount30d)
	}
	if len(defResp.RestrictedExcluded) != 0 {
		t.Errorf("tree restricted_excluded=%v, want empty", defResp.RestrictedExcluded)
	}

	// explicit scope=self returns only root's own figures
	recSelf := doRollupGet(t, h, ws, viewer, root, "self")
	if recSelf.Code != http.StatusOK {
		t.Fatalf("self scope: status=%d body=%s", recSelf.Code, recSelf.Body)
	}
	selfResp := decodeRollupHTTP(t, recSelf)
	if selfResp.Scope != "self" {
		t.Errorf("self body.scope=%q, want self", selfResp.Scope)
	}
	if selfResp.AggregatedAccountCount != 1 {
		t.Errorf("self aggregated_account_count=%d, want 1", selfResp.AggregatedAccountCount)
	}
	if selfResp.WeightedPipeline.AmountMinor == nil || *selfResp.WeightedPipeline.AmountMinor != 10000 {
		t.Errorf("self weighted_pipeline.amount_minor=%v, want 10000", selfResp.WeightedPipeline.AmountMinor)
	}
	if selfResp.ActivityCount30d != 1 {
		t.Errorf("self activity_count_30d=%d, want 1", selfResp.ActivityCount30d)
	}
	if len(selfResp.RestrictedExcluded) != 0 {
		t.Errorf("self restricted_excluded=%v, want empty", selfResp.RestrictedExcluded)
	}

	// explicit scope=tree matches the default
	recTree := doRollupGet(t, h, ws, viewer, root, "tree")
	if recTree.Code != http.StatusOK {
		t.Fatalf("explicit tree scope: status=%d", recTree.Code)
	}
	treeResp := decodeRollupHTTP(t, recTree)
	if treeResp.AggregatedAccountCount != defResp.AggregatedAccountCount ||
		treeResp.ActivityCount30d != defResp.ActivityCount30d {
		t.Errorf("explicit tree != default: count %d vs %d, activity %d vs %d",
			treeResp.AggregatedAccountCount, defResp.AggregatedAccountCount,
			treeResp.ActivityCount30d, defResp.ActivityCount30d)
	}
}

// TestHierarchyRollupHTTP_RestrictedNodeAndGrant proves RBAC-honest restricted-node
// exclusion (RD-AC-1) at the HTTP layer: a viewer with row_scope=own sees an unreadable
// child in restricted_excluded; adding a record_grant flips it into the included set.
func TestHierarchyRollupHTTP_RestrictedNodeAndGrant(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, 100)
	today := time.Now().UTC()
	seedFXRate(t, db, ws, "USD", "EUR", "1.0000000000", today)
	viewer := seedUserWithRowScope(t, db, ws, "own")
	other := seedUserWithRowScope(t, db, ws, "own")

	// root owned by viewer; child owned by other (viewer can't read child without a grant).
	root := seedOrgWithParent(t, db, ws, "root", nil, &viewer)
	child := seedOrgWithParent(t, db, ws, "child", &root, &other)

	usdAmt := func(v int64) *int64 { v2 := v; return &v2 }
	fx := "1.0000000000"
	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, usdAmt(5000), &fx)
	seedOpenDeal(t, db, ws, pipelineID, stageID, &child, usdAmt(9000), &fx)

	h := newRollupHTTPHandler(db)

	// Before grant: child in restricted_excluded, its 9000 excluded from total.
	rec := doRollupGet(t, h, ws, viewer, root, "tree")
	if rec.Code != http.StatusOK {
		t.Fatalf("before-grant: status=%d body=%s", rec.Code, rec.Body)
	}
	before := decodeRollupHTTP(t, rec)
	if len(before.RestrictedExcluded) != 1 || before.RestrictedExcluded[0].ID != child {
		t.Errorf("before-grant restricted_excluded=%+v, want [{id=%s}]", before.RestrictedExcluded, child)
	}
	if before.RestrictedExcluded[0].DisplayName != "child" {
		t.Errorf("before-grant restricted display_name=%q, want child", before.RestrictedExcluded[0].DisplayName)
	}
	if before.WeightedPipeline.AmountMinor == nil || *before.WeightedPipeline.AmountMinor != 5000 {
		t.Errorf("before-grant weighted_pipeline=%v, want 5000 (root only)", before.WeightedPipeline.AmountMinor)
	}
	if before.AggregatedAccountCount != 1 {
		t.Errorf("before-grant aggregated_account_count=%d, want 1 (root only)", before.AggregatedAccountCount)
	}

	// Add record_grant for viewer on child; child flips into the included set.
	seedRecordGrant(t, db, ws, child, viewer)

	rec2 := doRollupGet(t, h, ws, viewer, root, "tree")
	if rec2.Code != http.StatusOK {
		t.Fatalf("after-grant: status=%d body=%s", rec2.Code, rec2.Body)
	}
	after := decodeRollupHTTP(t, rec2)
	if len(after.RestrictedExcluded) != 0 {
		t.Errorf("after-grant restricted_excluded=%+v, want empty", after.RestrictedExcluded)
	}
	if after.WeightedPipeline.AmountMinor == nil || *after.WeightedPipeline.AmountMinor != 14000 {
		t.Errorf("after-grant weighted_pipeline=%v, want 14000 (root+child)", after.WeightedPipeline.AmountMinor)
	}
	if after.AggregatedAccountCount != 2 {
		t.Errorf("after-grant aggregated_account_count=%d, want 2", after.AggregatedAccountCount)
	}
}

// TestHierarchyRollupHTTP_FXRateUnavailable proves a missing stored FX rate for an open
// deal returns 422 with code=fx_rate_unavailable and the correct details shape.
func TestHierarchyRollupHTTP_FXRateUnavailable(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, 50)
	viewer := seedUserWithRowScope(t, db, ws, "all")

	root := seedOrgWithParent(t, db, ws, "root", nil, nil)
	// USD open deal — no fx_rate table row seeded for USD→EUR.
	usdAmt := int64(100000)
	fx := "1.0000000000"
	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, &usdAmt, &fx)

	h := newRollupHTTPHandler(db)
	rec := doRollupGet(t, h, ws, viewer, root, "tree")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("fx unavailable: status=%d, want 422; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Code    string `json:"code"`
		Details struct {
			Currency string `json:"currency"`
			AsOf     string `json:"as_of"`
		} `json:"details"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode 422 body: %v (body=%s)", err, rec.Body)
	}
	if body.Code != "fx_rate_unavailable" {
		t.Errorf("422 code=%q, want fx_rate_unavailable", body.Code)
	}
	if body.Details.Currency != "USD" {
		t.Errorf("422 details.currency=%q, want USD", body.Details.Currency)
	}
	if body.Details.AsOf == "" {
		t.Errorf("422 details.as_of is empty, want a YYYY-MM-DD date")
	}
}

// TestHierarchyRollupHTTP_NotFound proves a nonexistent org id and an org from a
// different workspace both return 404.
func TestHierarchyRollupHTTP_NotFound(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws1 := pgtest.NewWorkspaceSQL(t, db)
	ws2 := pgtest.NewWorkspaceSQL(t, db)

	// Seed an org in ws2 (temporarily set RLS to ws2 for the insert).
	pgtest.SetRLS(t, db, ws2)
	ws2Org := seedOrgWithParent(t, db, ws2, "ws2-org", nil, nil)

	// Switch to ws1 for the viewer and all subsequent operations.
	pgtest.SetRLS(t, db, ws1)
	viewer := seedUserWithRowScope(t, db, ws1, "all")

	h := newRollupHTTPHandler(db)

	// Nonexistent UUID → 404 not_found.
	rec1 := doRollupGet(t, h, ws1, viewer, "00000000-0000-0000-0000-000000000000", "tree")
	if rec1.Code != http.StatusNotFound {
		t.Errorf("nonexistent org: status=%d, want 404", rec1.Code)
	}
	var body1 struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec1.Body.Bytes(), &body1)
	if body1.Code != "not_found" {
		t.Errorf("nonexistent org body.code=%q, want not_found", body1.Code)
	}

	// Org from ws2 requested under ws1 — CTE's workspace_id filter excludes it → 404.
	rec2 := doRollupGet(t, h, ws1, viewer, ws2Org, "tree")
	if rec2.Code != http.StatusNotFound {
		t.Errorf("out-of-workspace org: status=%d, want 404", rec2.Code)
	}
}

// TestHierarchyRollupHTTP_Decomposition verifies RD-AC-8's decomposition claim at the
// HTTP response level: the tree-scope total for a node equals its own self-scope figures
// plus the sum of its readable children's tree-scope figures. Called multiple times on
// the same seeded tree and reconciled arithmetically.
func TestHierarchyRollupHTTP_Decomposition(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	setStageProbability(t, db, stageID, 100)
	today := time.Now().UTC()
	seedFXRate(t, db, ws, "USD", "EUR", "1.0000000000", today)
	viewer := seedUserWithRowScope(t, db, ws, "all")

	// 3-level tree: root → childA → grandchild
	root := seedOrgWithParent(t, db, ws, "root", nil, nil)
	childA := seedOrgWithParent(t, db, ws, "childA", &root, nil)
	grandchild := seedOrgWithParent(t, db, ws, "grandchild", &childA, nil)

	usdAmt := func(v int64) *int64 { v2 := v; return &v2 }
	fx := "1.0000000000"
	seedOpenDeal(t, db, ws, pipelineID, stageID, &root, usdAmt(3000), &fx)
	seedActivity(t, db, ws, root, today.Add(-1*24*time.Hour))
	seedOpenDeal(t, db, ws, pipelineID, stageID, &childA, usdAmt(5000), &fx)
	seedActivity(t, db, ws, childA, today.Add(-2*24*time.Hour))
	seedOpenDeal(t, db, ws, pipelineID, stageID, &grandchild, usdAmt(7000), &fx)
	seedActivity(t, db, ws, grandchild, today.Add(-3*24*time.Hour))

	h := newRollupHTTPHandler(db)

	get := func(orgID, scope string) rollupWire {
		t.Helper()
		rec := doRollupGet(t, h, ws, viewer, orgID, scope)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s scope=%s: status=%d body=%s", orgID, scope, rec.Code, rec.Body)
		}
		return decodeRollupHTTP(t, rec)
	}
	moneyVal := func(m rollupMoneyWire) int64 {
		if m.AmountMinor == nil {
			return 0
		}
		return *m.AmountMinor
	}

	rootTree := get(root, "tree")
	rootSelf := get(root, "self")
	childATree := get(childA, "tree") // includes grandchild

	// RD-AC-8: root_tree = root_self + childA_tree
	wantW := moneyVal(rootSelf.WeightedPipeline) + moneyVal(childATree.WeightedPipeline)
	if moneyVal(rootTree.WeightedPipeline) != wantW {
		t.Errorf("decomposition (weighted): root_tree=%d, want root_self(%d)+childA_tree(%d)=%d",
			moneyVal(rootTree.WeightedPipeline),
			moneyVal(rootSelf.WeightedPipeline), moneyVal(childATree.WeightedPipeline), wantW)
	}
	wantA := rootSelf.ActivityCount30d + childATree.ActivityCount30d
	if rootTree.ActivityCount30d != wantA {
		t.Errorf("decomposition (activity): root_tree=%d, want root_self(%d)+childA_tree(%d)=%d",
			rootTree.ActivityCount30d, rootSelf.ActivityCount30d, childATree.ActivityCount30d, wantA)
	}

	// Absolute values: 3000 + 5000 + 7000 = 15000 total; 3 nodes.
	if moneyVal(rootTree.WeightedPipeline) != 15000 {
		t.Errorf("root tree weighted_pipeline=%d, want 15000", moneyVal(rootTree.WeightedPipeline))
	}
	if rootTree.AggregatedAccountCount != 3 {
		t.Errorf("root tree aggregated_account_count=%d, want 3", rootTree.AggregatedAccountCount)
	}
	if rootTree.ActivityCount30d != 3 {
		t.Errorf("root tree activity_count_30d=%d, want 3", rootTree.ActivityCount30d)
	}
}

// TestHierarchyRollupHTTP_Auth confirms the pre-existing RbacMiddleware gate fires
// correctly for the /hierarchy-rollup sub-path: 401 with no session, 403 for a role
// with no organization.read permission.
func TestHierarchyRollupHTTP_Auth(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	org := seedOrgWithParent(t, db, ws, "auth-test-org", nil, nil)
	noOrgUser := seedHTTPUserNoOrgPerm(t, db, ws)

	// Wrap with RbacMiddleware to engage the auth gates.
	h := crmauth.RbacMiddleware(db, "organization")(newRollupHTTPHandler(db))

	// 401: no principal in context — RequireAuth fires before permissions are checked.
	req401 := httptest.NewRequest(http.MethodGet, "/organizations/"+org+"/hierarchy-rollup", nil)
	rec401 := httptest.NewRecorder()
	h.ServeHTTP(rec401, req401)
	if rec401.Code != http.StatusUnauthorized {
		t.Errorf("no-session: status=%d, want 401; body=%s", rec401.Code, rec401.Body)
	}

	// 403: principal exists but role has no organization.read permission.
	req403 := httptest.NewRequest(http.MethodGet, "/organizations/"+org+"/hierarchy-rollup", nil)
	req403 = req403.WithContext(crmctx.With(req403.Context(), crmctx.Principal{TenantID: ws, UserID: noOrgUser}))
	rec403 := httptest.NewRecorder()
	h.ServeHTTP(rec403, req403)
	if rec403.Code != http.StatusForbidden {
		t.Errorf("no-org-perm: status=%d, want 403; body=%s", rec403.Code, rec403.Body)
	}
}
