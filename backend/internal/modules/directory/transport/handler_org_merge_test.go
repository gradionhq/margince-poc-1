//go:build integration

package transport

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedWorkspace mirrors modules/directory's helpers_shared_test.go helper of
// the same name for the merge test fixtures this file adds; package
// transport (directory) has no existing per-test dynamic-workspace seeder
// (its sibling handler test files each seed one fixed shared workspace ID
// instead), so a dynamic-ws version is added here, scoped to this file.
func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'t06-merge-org-ws',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "t06-merge-org-"+wsID); err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// seedAppUser mirrors modules/directory's helpers_shared_test.go helper of
// the same name: audit_log.on_behalf_of FKs to app_user(id), so an agent
// principal's UserID must be a seeded app_user row, not an arbitrary string.
func seedAppUser(t *testing.T, db *sql.DB, id, wsID string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO app_user(id,workspace_id,email,display_name,is_agent) VALUES($1::uuid,$2::uuid,$3,'Agent Test',true) ON CONFLICT DO NOTHING`,
		id, wsID, "merge-agent-"+id+"@example.com",
	); err != nil {
		t.Fatal("seed app_user:", err)
	}
}

func createTestOrg(t *testing.T, store *crmcore.OrgStore, ws, name string) crmcore.Organization {
	t.Helper()
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:test", TenantID: ws})
	created, err := store.Create(ctx, crmcore.Organization{
		WorkspaceID: ws, DisplayName: name, Source: "test", CapturedBy: "human:test",
	})
	if err != nil {
		t.Fatalf("create test org %s: %v", name, err)
	}
	return created
}

// UAT step 2 (org mirror): human principal merges without a token -> 200,
// loser archived, merged_into_id set.
func TestMergeOrgHumanNoTokenSucceeds(t *testing.T) {
	db := openDealTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := crmcore.NewOrgStore(db)
	handler := orgHandlerForTest(db, store)

	loser := createTestOrg(t, store, ws, "Loser Co")
	target := createTestOrg(t, store, ws, "Target Co")

	req := httptest.NewRequest(http.MethodPost, "/organizations/"+loser.ID+"/merge",
		strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "human:t", TenantID: ws, IsAgent: false}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

// UAT step 5 (org mirror): agent principal without a token -> 403
// approval_required; with a valid single-use token -> 200; reusing the same
// token -> fails. Tool remains "merge_records" — the SAME contract verb as
// the person mirror, not "merge_organization" (Global Constraints; Task 4
// Step 6); disambiguation is via diff_hash's organization_id/target_id.
func TestMergeOrgAgentRequiresApprovalToken(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	db := openDealTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := crmcore.NewOrgStore(db)
	handler := orgHandlerForTest(db, store)
	loser := createTestOrg(t, store, ws, "Loser Co")
	target := createTestOrg(t, store, ws, "Target Co")
	agentUserID := ids.New()
	seedAppUser(t, db, agentUserID, ws)
	agentCtx := crmctx.With(context.Background(), crmctx.Principal{UserID: agentUserID, TenantID: ws, IsAgent: true})

	// No token.
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req = req.WithContext(agentCtx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-token status = %d, want 403", w.Code)
	}

	diffHash := crmapprovals.HashDiff(map[string]any{"organization_id": loser.ID, "target_id": target.ID})
	token, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: ids.New(), WorkspaceID: ws, Tool: "merge_records", DiffHash: diffHash,
		Exp: time.Now().Add(time.Hour), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/organizations/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req2 = req2.WithContext(agentCtx)
	req2.Header.Set("X-Approval-Token", token)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("valid-token status = %d, body = %s", w2.Code, w2.Body.String())
	}

	// Replay the identical token — must be rejected even though the org pair
	// is now already merged, proving single-use consumption, not just a
	// diff_hash mismatch.
	req3 := httptest.NewRequest(http.MethodPost, "/organizations/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req3 = req3.WithContext(agentCtx)
	req3.Header.Set("X-Approval-Token", token)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("replayed-token status = %d, want 403", w3.Code)
	}
}

// UAT step 4 / PO-AC-M3 (org mirror): self-merge -> 422 validation_error.
func TestMergeOrgSelfMerge422(t *testing.T) {
	db := openDealTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := crmcore.NewOrgStore(db)
	handler := orgHandlerForTest(db, store)
	o := createTestOrg(t, store, ws, "Solo Co")

	req := httptest.NewRequest(http.MethodPost, "/organizations/"+o.ID+"/merge", strings.NewReader(`{"target_id":"`+o.ID+`"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "human:t", TenantID: ws}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("self-merge status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

// UAT step 3 (org mirror): merging an already-merged organization -> 422
// with pointer to survivor.
func TestMergeOrgAlreadyMerged422WithPointer(t *testing.T) {
	db := openDealTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := crmcore.NewOrgStore(db)
	handler := orgHandlerForTest(db, store)
	a, b, c := createTestOrg(t, store, ws, "A Co"), createTestOrg(t, store, ws, "B Co"), createTestOrg(t, store, ws, "C Co")
	if _, err := store.Merge(crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws}), a.ID, b.ID, ws); err != nil {
		t.Fatalf("seed merge a->b: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/organizations/"+a.ID+"/merge", strings.NewReader(`{"target_id":"`+c.ID+`"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "human:t", TenantID: ws}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("already-merged status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), b.ID) {
		t.Fatalf("422 body must point at the actual survivor %s, got %s", b.ID, w.Body.String())
	}
}

// PO-AC-M5 at the HTTP layer (org mirror): two concurrent merge requests for
// the same loser — one 200s, the other maps ErrVersionSkew to 409
// version_skew.
func TestMergeOrgConcurrent409VersionSkew(t *testing.T) {
	db := openDealTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := crmcore.NewOrgStore(db)
	handler := orgHandlerForTest(db, store)
	loser := createTestOrg(t, store, ws, "Loser Co")
	targetB := createTestOrg(t, store, ws, "TargetB Co")
	targetC := createTestOrg(t, store, ws, "TargetC Co")
	humanCtx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})

	fire := func(targetID string) int {
		req := httptest.NewRequest(http.MethodPost, "/organizations/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+targetID+`"}`))
		req = req.WithContext(humanCtx)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}
	results := make(chan int, 2)
	go func() { results <- fire(targetB.ID) }()
	go func() { results <- fire(targetC.ID) }()
	r1, r2 := <-results, <-results

	successes, conflicts := 0, 0
	for _, code := range []int{r1, r2} {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent-merge status: %d", code)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent merge HTTP results: got %d OK / %d Conflict, want exactly 1/1", successes, conflicts)
	}
}
