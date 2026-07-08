//go:build integration

package transport

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

// init registers the "target_stage_semantic" dynamic-tier resolver that, in
// the running server, is only wired up at cmd/api composition
// (routes.go: toolgate.RegisterResolver("target_stage_semantic",
// deals.ResolveDynamicTier)). This test package never goes through that
// composition, so without this the resolver registry has no entry for
// advanceDealTool's Resolver and toolgate's effectiveTier floors every
// transition to 🟡 (never escaping to 🟢 by omission) — breaking UAT1's
// open->open expectation of no approval being required.
func init() {
	toolgate.RegisterResolver("target_stage_semantic", deals.ResolveDynamicTier)
}

const advHandlerTestWorkspaceID = "00000000-0000-0000-0000-000000000a99"

// advHandlerTestUserID is a valid UUID-format user ID used for human callers
// in handler integration tests (audit_log.on_behalf_of requires UUID or NULL).
const advHandlerTestUserID = "00000000-0000-0000-0000-0000000a0001"

// advHandlerTestAgentID is a valid UUID-format user ID used for agent callers.
const advHandlerTestAgentID = "00000000-0000-0000-0000-0000000a0002"

func withAdvWorkspace(r *http.Request, isAgent bool, userID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: advHandlerTestWorkspaceID, UserID: userID, IsAgent: isAgent})
	return r.WithContext(ctx)
}

// seedAdvHandlerFixtures seeds one pipeline with two open stages (openA,
// openB) plus a won and a lost stage, and one EUR->USD fx_rate row so a
// close can freeze a real rate. It also seeds an app_user for the agent test
// caller so that audit_log.on_behalf_of (which FKs to app_user.id) is valid.
func seedAdvHandlerFixtures(t *testing.T, db *sql.DB, tag string) (pipelineID, openA, openB, won, lost string) {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t12-adv-h-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, advHandlerTestWorkspaceID, "t12-adv-h-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	// Seed agent app_user so audit_log.on_behalf_of FK is satisfied for agent callers.
	if _, err := db.Exec(`INSERT INTO app_user (id, workspace_id, email, display_name, is_agent)
		VALUES ($1, $2, 't12-agent@test.example', 'T12 Agent', true)
		ON CONFLICT (id) DO NOTHING`,
		advHandlerTestAgentID, advHandlerTestWorkspaceID); err != nil {
		t.Fatalf("seed agent user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1,'EUR','USD',1.08,current_date) ON CONFLICT DO NOTHING`, advHandlerTestWorkspaceID); err != nil {
		t.Fatalf("seed fx_rate: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, advHandlerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		advHandlerTestWorkspaceID, "P-"+tag).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	seed := func(name string, pos int, semantic string, prob int) string {
		var id string
		if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
			VALUES (uuidv7(), $1, $2, $3, $4, $5, $6) RETURNING id`,
			advHandlerTestWorkspaceID, pipelineID, name+"-"+tag, pos, semantic, prob).Scan(&id); err != nil {
			t.Fatalf("seed stage %s: %v", name, err)
		}
		return id
	}
	openA = seed("open-a", 1, "open", 10)
	openB = seed("open-b", 2, "open", 40)
	won = seed("won", 3, "won", 100)
	lost = seed("lost", 4, "lost", 0)
	return pipelineID, openA, openB, won, lost
}

func createAdvHandlerDeal(t *testing.T, h *DealHandler, pipelineID, stageID string) map[string]any {
	t.Helper()
	body := map[string]any{
		"name": "adv-deal", "pipeline_id": pipelineID, "stage_id": stageID,
		"amount_minor": 5000, "currency": "EUR",
		"source": "test", "captured_by": "human:test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(b))
	req = withAdvWorkspace(req, false, "human:test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create deal: %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created deal: %v", err)
	}
	return created
}

func postAdvance(t *testing.T, h *DealHandler, dealID string, body map[string]any, isAgent bool, token string) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/deals/"+dealID+"/advance", bytes.NewReader(b))
	// audit_log.on_behalf_of is cast to ::uuid — use valid UUID user IDs.
	userID := advHandlerTestUserID
	if isAgent {
		userID = advHandlerTestAgentID
	}
	req = withAdvWorkspace(req, isAgent, userID)
	if token != "" {
		req.Header.Set("X-Approval-Token", token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAdvanceDeal_UAT1_OpenToOpen_NoTokenNeeded(t *testing.T) {
	db := openDealTestDB(t)
	h := dealHandlerForTest(db, adapters.NewDealStore(db))
	pipelineID, openA, openB, _, _ := seedAdvHandlerFixtures(t, db, "uat1")
	created := createAdvHandlerDeal(t, h, pipelineID, openA)
	dealID := created["id"].(string)

	// Agent principal, open->open, no token — must still succeed (green).
	w := postAdvance(t, h, dealID, map[string]any{"to_stage_id": openB}, true, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var historyCount, auditCount, eventCount int
	_ = db.QueryRow(`SELECT count(*) FROM deal_stage_history WHERE deal_id=$1 AND to_stage_id=$2`, dealID, openB).Scan(&historyCount)
	_ = db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity_id=$1 AND action='advance_stage'`, dealID).Scan(&auditCount)
	_ = db.QueryRow(`SELECT count(*) FROM event_outbox WHERE entity_id=$1 AND topic='deal.stage_changed'`, dealID).Scan(&eventCount)
	if historyCount != 1 || auditCount != 1 || eventCount != 1 {
		t.Fatalf("expected exactly 1 row each (history=%d audit=%d event=%d)", historyCount, auditCount, eventCount)
	}
}

func TestAdvanceDeal_UAT2_AgentWithoutToken_403ApprovalRequired(t *testing.T) {
	db := openDealTestDB(t)
	h := dealHandlerForTest(db, adapters.NewDealStore(db))
	pipelineID, openA, _, won, _ := seedAdvHandlerFixtures(t, db, "uat2")
	created := createAdvHandlerDeal(t, h, pipelineID, openA)
	dealID := created["id"].(string)
	preVersion := created["version"]

	w := postAdvance(t, h, dealID, map[string]any{"to_stage_id": won, "status": "won"}, true, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "approval_required" {
		t.Fatalf("expected code=approval_required, got %v", body["code"])
	}

	var status string
	var version float64
	_ = db.QueryRow(`SELECT status, version FROM deal WHERE id=$1`, dealID).Scan(&status, &version)
	if status != "open" || version != preVersion.(float64) {
		t.Fatalf("expected no mutation: status=%q version=%v (was %v)", status, version, preVersion)
	}
}

func TestAdvanceDeal_UAT3_AgentWithValidToken_SucceedsThenReplayRejected(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "handler-it-secret")
	db := openDealTestDB(t)
	h := dealHandlerForTest(db, adapters.NewDealStore(db))
	pipelineID, openA, _, won, _ := seedAdvHandlerFixtures(t, db, "uat3")
	created := createAdvHandlerDeal(t, h, pipelineID, openA)
	dealID := created["id"].(string)
	version := int64(created["version"].(float64))

	// diffFields must match checkApprovalGate's shape exactly (deal_id,
	// to_stage_id, status, from_semantic, to_semantic) since HashDiff hashes
	// the whole map — openA->won: from_semantic="open", to_semantic="won".
	diffFields := map[string]any{
		"deal_id": dealID, "to_stage_id": won, "status": "won",
		"from_semantic": "open", "to_semantic": "won",
	}
	diffHash := approvalsport.HashDiff(diffFields)
	tok, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: "uat3-jti-" + dealID, ApprovalID: "appr-uat3", WorkspaceID: advHandlerTestWorkspaceID,
		Tool: "advance_deal", DiffHash: diffHash, TargetVersion: &version,
		Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	body := map[string]any{"to_stage_id": won, "status": "won"}
	w := postAdvance(t, h, dealID, body, true, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on first use, got %d: %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated["closed_at"] == nil || updated["fx_rate_to_base"] == nil {
		t.Fatalf("expected closed_at/fx_rate_to_base set: %+v", updated)
	}

	// Replay: same token, same body — must be rejected even though the deal
	// is already at "won" (idempotent-looking request, non-idempotent token).
	w2 := postAdvance(t, h, dealID, body, true, tok)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on replay, got %d: %s", w2.Code, w2.Body.String())
	}
	var body2 map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &body2)
	if body2["code"] != "approval_token_invalid" {
		t.Fatalf("expected code=approval_token_invalid on replay, got %v", body2["code"])
	}
}

func TestAdvanceDeal_UAT4_LostWithoutReason_422(t *testing.T) {
	db := openDealTestDB(t)
	h := dealHandlerForTest(db, adapters.NewDealStore(db))
	pipelineID, openA, _, _, lost := seedAdvHandlerFixtures(t, db, "uat4")
	created := createAdvHandlerDeal(t, h, pipelineID, openA)
	dealID := created["id"].(string)

	// Human principal — no token required for the yellow gate, but lost_reason
	// validation still applies.
	w := postAdvance(t, h, dealID, map[string]any{"to_stage_id": lost}, false, "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdvanceDeal_UAT5_Reopen_AgentGatedThenClearsOnSuccess(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "handler-it-secret")
	db := openDealTestDB(t)
	h := dealHandlerForTest(db, adapters.NewDealStore(db))
	pipelineID, openA, _, _, lost := seedAdvHandlerFixtures(t, db, "uat5")
	created := createAdvHandlerDeal(t, h, pipelineID, openA)
	dealID := created["id"].(string)

	reason := "lost to competitor"
	wClose := postAdvance(t, h, dealID, map[string]any{"to_stage_id": lost, "lost_reason": reason}, false, "")
	if wClose.Code != http.StatusOK {
		t.Fatalf("expected close to succeed, got %d: %s", wClose.Code, wClose.Body.String())
	}
	var closedDeal map[string]any
	_ = json.Unmarshal(wClose.Body.Bytes(), &closedDeal)
	closedVersion := int64(closedDeal["version"].(float64))

	// Agent reopen without a token -> 403.
	wNoToken := postAdvance(t, h, dealID, map[string]any{"to_stage_id": openA}, true, "")
	if wNoToken.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on agent reopen without token, got %d: %s", wNoToken.Code, wNoToken.Body.String())
	}

	// Reopen from the "lost" stage back to openA: from_semantic="lost",
	// to_semantic="open" — must match checkApprovalGate's diffFields shape
	// exactly since HashDiff hashes the whole map.
	diffFields := map[string]any{
		"deal_id": dealID, "to_stage_id": openA, "status": "open",
		"from_semantic": "lost", "to_semantic": "open",
	}
	diffHash := approvalsport.HashDiff(diffFields)
	tok, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: "uat5-jti-" + dealID, WorkspaceID: advHandlerTestWorkspaceID,
		Tool: "advance_deal", DiffHash: diffHash, TargetVersion: &closedVersion,
		Exp: time.Now().Add(5 * time.Minute), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	wReopen := postAdvance(t, h, dealID, map[string]any{"to_stage_id": openA}, true, tok)
	if wReopen.Code != http.StatusOK {
		t.Fatalf("expected 200 on agent reopen with valid token, got %d: %s", wReopen.Code, wReopen.Body.String())
	}
	var reopened map[string]any
	_ = json.Unmarshal(wReopen.Body.Bytes(), &reopened)
	if reopened["closed_at"] != nil || reopened["lost_reason"] != nil || reopened["fx_rate_to_base"] != nil {
		t.Fatalf("expected closed_at/lost_reason/fx cleared on reopen: %+v", reopened)
	}
}

func TestAdvanceDeal_UAT6_RenamedStageSemanticStillGoverned(t *testing.T) {
	db := openDealTestDB(t)
	h := dealHandlerForTest(db, adapters.NewDealStore(db))
	pipelineID, openA, _, won, _ := seedAdvHandlerFixtures(t, db, "uat6")
	created := createAdvHandlerDeal(t, h, pipelineID, openA)
	dealID := created["id"].(string)

	// Rename the won stage's NAME only — its semantic ('won') is untouched.
	if _, err := db.Exec(`UPDATE stage SET name='Definitely Not Closed' WHERE id=$1`, won); err != nil {
		t.Fatalf("rename stage: %v", err)
	}

	w := postAdvance(t, h, dealID, map[string]any{"to_stage_id": won, "status": "won"}, true, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("renaming the stage must not dodge the gate: expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
