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
	directory "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

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

func createTestPerson(t *testing.T, store *directory.PersonStore, ws, name string) directory.Person {
	t.Helper()
	p := directory.NewPerson(name, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	p.WorkspaceID = ws
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:test", TenantID: ws})
	created, err := store.Create(ctx, p, nil)
	if err != nil {
		t.Fatalf("create test person %s: %v", name, err)
	}
	return created
}

// UAT step 2: human principal merges without a token -> 200, loser archived,
// merged_into_id set.
func TestMergePersonHumanNoTokenSucceeds(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := directory.NewPersonStore(db)
	handler := personHandlerForTest(db, store)

	loser := createTestPerson(t, store, ws, "Loser")
	target := createTestPerson(t, store, ws, "Target")

	req := httptest.NewRequest(http.MethodPost, "/people/"+loser.ID+"/merge",
		strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "human:t", TenantID: ws, IsAgent: false}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

// UAT step 5: agent principal without a token -> 403 approval_required;
// with a valid single-use token -> 200; reusing the same token -> fails.
func TestMergePersonAgentRequiresApprovalToken(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := directory.NewPersonStore(db)
	handler := personHandlerForTest(db, store)
	loser := createTestPerson(t, store, ws, "Loser")
	target := createTestPerson(t, store, ws, "Target")
	agentUserID := ids.New()
	seedAppUser(t, db, agentUserID, ws)
	agentCtx := crmctx.With(context.Background(), crmctx.Principal{UserID: agentUserID, TenantID: ws, IsAgent: true})

	// No token.
	req := httptest.NewRequest(http.MethodPost, "/people/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req = req.WithContext(agentCtx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-token status = %d, want 403", w.Code)
	}

	// Valid token — Tool MUST be the contract's declared x-mcp-tool verb
	// ("merge_records", crm.yaml:335/557), never a per-entity string like
	// "merge_person"/"merge_organization": a real minted token carries the
	// declared verb, and VerifyAndConsume does an exact-string match on Tool
	// (token.go:146). Person vs. org is disambiguated by diff_hash alone.
	diffHash := crmapprovals.HashDiff(map[string]any{"person_id": loser.ID, "target_id": target.ID})
	token, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: ids.New(), WorkspaceID: ws, Tool: "merge_records", DiffHash: diffHash,
		Exp: time.Now().Add(time.Hour), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/people/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req2 = req2.WithContext(agentCtx)
	req2.Header.Set("X-Approval-Token", token)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("valid-token status = %d, body = %s", w2.Code, w2.Body.String())
	}

	// Replay the identical token against the identical (now-already-merged)
	// pair — UAT step 5's literal "reusing the same token fails (already
	// consumed)". VerifyAndConsume rejects on the jti already being consumed
	// before the store call ever runs, so this proves single-use, not just a
	// diff_hash mismatch against a different pair.
	req3 := httptest.NewRequest(http.MethodPost, "/people/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+target.ID+`"}`))
	req3 = req3.WithContext(agentCtx)
	req3.Header.Set("X-Approval-Token", token)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("replayed-token status = %d, want 403", w3.Code)
	}
}

// UAT step 4 / PO-AC-M3: self-merge -> 422 validation_error.
func TestMergePersonSelfMerge422(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := directory.NewPersonStore(db)
	handler := personHandlerForTest(db, store)
	p := createTestPerson(t, store, ws, "Solo")

	req := httptest.NewRequest(http.MethodPost, "/people/"+p.ID+"/merge", strings.NewReader(`{"target_id":"`+p.ID+`"}`))
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{UserID: "human:t", TenantID: ws}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("self-merge status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

// UAT step 3: merging an already-merged person -> 422 with pointer to survivor.
func TestMergePersonAlreadyMerged422WithPointer(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := directory.NewPersonStore(db)
	handler := personHandlerForTest(db, store)
	a, b, c := createTestPerson(t, store, ws, "A"), createTestPerson(t, store, ws, "B"), createTestPerson(t, store, ws, "C")
	if _, err := store.Merge(crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws}), a.ID, b.ID, ws); err != nil {
		t.Fatalf("seed merge a->b: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/people/"+a.ID+"/merge", strings.NewReader(`{"target_id":"`+c.ID+`"}`))
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

// PO-AC-M5 at the HTTP layer: two concurrent merge requests for the same loser — one
// 200s, the other maps ErrVersionSkew to 409 version_skew. This is the only place the
// store-level ErrVersionSkew → HTTP 409 mapping (merge's Step 5 handler code) is actually
// proven; Task 2's store-level concurrency test only asserts the Go error value.
func TestMergePersonConcurrent409VersionSkew(t *testing.T) {
	db := openTestDB(t)
	ws := ids.New()
	seedWorkspace(t, db, ws)
	store := directory.NewPersonStore(db)
	handler := personHandlerForTest(db, store)
	loser := createTestPerson(t, store, ws, "Loser")
	targetB := createTestPerson(t, store, ws, "TargetB")
	targetC := createTestPerson(t, store, ws, "TargetC")
	humanCtx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human:t", TenantID: ws})

	fire := func(targetID string) int {
		req := httptest.NewRequest(http.MethodPost, "/people/"+loser.ID+"/merge", strings.NewReader(`{"target_id":"`+targetID+`"}`))
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
