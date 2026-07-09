//go:build integration

package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'op-t06-it',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "op-t06-"+wsID); err != nil {
		t.Fatal("seed workspace:", err)
	}
}

func offerHandlerForTest(db *sql.DB) *OfferHandler {
	return NewOfferHandler(adapters.NewOfferStore(db).WithDealStore(deals.NewDealStore(db)), adapters.NewOfferLineItemStore(db, adapters.NewProductStore(db)), &crmapprovals.DBVerifier{DB: db}, blobstore.NewMemoryStore(), NewNoOpRetriever())
}

func withOfferWorkspace(r *http.Request, wsID, userID string) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: wsID, UserID: userID})
	return r.WithContext(ctx)
}

func seedOfferWorkspace(t *testing.T, db *sql.DB, wsID string) (buyerOrgID, dealID, humanID, agentID string) {
	t.Helper()
	seedWorkspace(t, db, wsID)

	humanID = ids.New()
	agentID = ids.New()
	orgID := ids.New()
	pipelineID := ids.New()
	stageID := ids.New()
	dealID = ids.New()

	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: humanID, TenantID: wsID})
	if err := database.WithWorkspaceTx(ctx, db, wsID, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app_user(id,workspace_id,email,display_name) VALUES($1::uuid,$2::uuid,$3,'Human Test') ON CONFLICT DO NOTHING`,
			humanID, wsID, "offer-human-"+humanID+"@example.com"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app_user(id,workspace_id,email,display_name,is_agent) VALUES($1::uuid,$2::uuid,$3,'Agent Test',true) ON CONFLICT DO NOTHING`,
			agentID, wsID, "offer-agent-"+agentID+"@example.com"); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO organization(id,workspace_id,name,address,source,captured_by,version)
			 VALUES($1::uuid,$2::uuid,$3,$4::jsonb,$5,$6,1) RETURNING id`,
			orgID, wsID, "OP-T06 Buyer GmbH", `{"city":"Berlin","street":"Teststrasse 1"}`, "uat", "human:op-t06").Scan(&buyerOrgID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO pipeline(id,workspace_id,name) VALUES($1::uuid,$2::uuid,$3) RETURNING id`,
			pipelineID, wsID, "OP-T06 Pipeline").Scan(&pipelineID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO stage(id,workspace_id,pipeline_id,name,position,semantic,win_probability)
			 VALUES($1::uuid,$2::uuid,$3::uuid,$4,1,'open',30) RETURNING id`,
			stageID, wsID, pipelineID, "Proposal").Scan(&stageID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO deal(id,workspace_id,name,pipeline_id,stage_id,organization_id,amount_minor,currency,source,captured_by,version)
			 VALUES($1::uuid,$2::uuid,$3,$4::uuid,$5::uuid,$6::uuid,0,'EUR',$7,$8,1) RETURNING id`,
			dealID, wsID, "OP-T06 UAT Deal", pipelineID, stageID, buyerOrgID, "uat", "human:op-t06").Scan(&dealID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed offer workspace: %v", err)
	}
	return buyerOrgID, dealID, humanID, agentID
}

func createDraftOffer(t *testing.T, h *OfferHandler, wsID, dealID, buyerOrgID, userID, currency string) string {
	t.Helper()
	body := map[string]any{
		"offer_number": "ANG-OP-T06-" + ids.New(),
		"currency":     currency,
		"buyer_org_id": buyerOrgID,
		"template_id":  nil,
		"source":       "uat",
		"captured_by":  "human:op-t06",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/deals/"+dealID+"/offers", bytes.NewReader(b))
	req = withOfferWorkspace(req, wsID, userID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertCreated201(t, w)
	resp := decodeJSONBody(t, w)
	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatalf("expected created offer id, got %v", resp)
	}
	return id
}

func addLineItem(t *testing.T, h *OfferHandler, wsID, offerID, userID string) {
	t.Helper()
	body := map[string]any{
		"position":         1,
		"description":      "Consulting package",
		"quantity":         2,
		"unit_price_minor": 125000,
		"discount_pct":     0,
		"tax_rate":         19,
		"source":           "uat",
		"captured_by":      "human:op-t06",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/line-items", bytes.NewReader(b))
	req = withOfferWorkspace(req, wsID, userID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertCreated201(t, w)
}

func getOffer(t *testing.T, h *OfferHandler, wsID, offerID, userID string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/offers/"+offerID, nil)
	req = withOfferWorkspace(req, wsID, userID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET offer status = %d, body=%s", w.Code, w.Body.String())
	}
	return decodeJSONBody(t, w)
}

func offerLineItems(t *testing.T, h *OfferHandler, wsID, offerID, userID string) []any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/offers/"+offerID+"/line-items", nil)
	req = withOfferWorkspace(req, wsID, userID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET line items status = %d, body=%s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	data, _ := resp["data"].([]any)
	return data
}

// setupSeededOfferWithLineItem is the shared "seed workspace + handler +
// draft offer + one line item" fixture chain used by nearly every send/
// regenerate test below.
func setupSeededOfferWithLineItem(t *testing.T, currency string) (db *sql.DB, h *OfferHandler, wsID, offerID, humanID, agentID string) {
	t.Helper()
	db = openTestDB(t)
	wsID = ids.New()
	buyerOrgID, dealID, humanID, agentID := seedOfferWorkspace(t, db, wsID)
	h = offerHandlerForTest(db)
	offerID = createDraftOffer(t, h, wsID, dealID, buyerOrgID, humanID, currency)
	addLineItem(t, h, wsID, offerID, humanID)
	return db, h, wsID, offerID, humanID, agentID
}

// signSendOfferToken builds and signs a single-use send_offer approval token
// for offerID, failing the test on any signing error.
func signSendOfferToken(t *testing.T, wsID, offerID string) string {
	t.Helper()
	diffHash := approvalsport.HashDiff(map[string]any{"offer_id": offerID})
	token, err := crmapprovals.SignToken(crmapprovals.TokenClaims{
		JTI: ids.New(), WorkspaceID: wsID, Tool: "send_offer", DiffHash: diffHash,
		Exp: time.Now().Add(time.Hour), SingleUse: true,
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

// sendOfferAsAgentExpect200 sends offerID as an agent principal bearing
// token, asserts a 200 response, and returns the decoded body.
func sendOfferAsAgentExpect200(t *testing.T, h *OfferHandler, wsID, agentID, offerID, token string) map[string]any {
	t.Helper()
	agentCtx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: agentID, IsAgent: true})
	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/send", nil)
	req = req.WithContext(agentCtx)
	req.Header.Set("X-Approval-Token", token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("agent send status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	return decodeJSONBody(t, w)
}

// UAT step 2: render a seeded draft offer with a real line item -> 200 and a
// blob-backed PDF asset ref.
func TestOfferHandler_Render_SetsRealPdfAssetRef(t *testing.T) {
	_, h, wsID, offerID, humanID, _ := setupSeededOfferWithLineItem(t, "EUR")

	got := getOffer(t, h, wsID, offerID, humanID)
	if net, ok := got["net_minor"].(float64); !ok || int64(net) != 250000 {
		t.Fatalf("expected net_minor=250000, got %v", got["net_minor"])
	}
	if tax, ok := got["tax_minor"].(float64); !ok || int64(tax) != 47500 {
		t.Fatalf("expected tax_minor=47500, got %v", got["tax_minor"])
	}
	if gross, ok := got["gross_minor"].(float64); !ok || int64(gross) != 297500 {
		t.Fatalf("expected gross_minor=297500, got %v", got["gross_minor"])
	}

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/render", nil)
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	ref, _ := resp["pdf_asset_ref"].(string)
	if ref == "" {
		t.Fatalf("expected pdf_asset_ref set, got %v", resp["pdf_asset_ref"])
	}

	rc, err := h.blob.Get(context.Background(), ref)
	if err != nil {
		t.Fatalf("blob get: %v", err)
	}
	defer rc.Close()
	pdfBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected a %%PDF- header, got %q", pdfBytes[:min(20, len(pdfBytes))])
	}
}

// UAT step 3: agent principal without a token -> 403 approval_required;
// with a valid single-use token -> 200 and frozen send fields populated.
func TestOfferHandler_Send_AgentNoToken_403_ThenValidToken_200(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	_, h, wsID, offerID, _, agentID := setupSeededOfferWithLineItem(t, "EUR")

	agentCtx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: agentID, IsAgent: true})

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/send", nil)
	req = req.WithContext(agentCtx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-token status = %d, want 403", w.Code)
	}
	resp := decodeJSONBody(t, w)
	if resp["code"] != "approval_required" {
		t.Fatalf("expected code=approval_required, got %v", resp["code"])
	}

	token := signSendOfferToken(t, wsID, offerID)

	resp2 := sendOfferAsAgentExpect200(t, h, wsID, agentID, offerID, token)
	if status, ok := resp2["status"].(string); !ok || status != "sent" {
		t.Fatalf("expected status=sent, got %v", resp2["status"])
	}
	if resp2["fx_rate_to_base"] == nil || resp2["fx_rate_date"] == nil || resp2["buyer_snapshot"] == nil || resp2["issuer_snapshot"] == nil {
		t.Fatalf("expected frozen send fields populated, got %+v", resp2)
	}

	req3 := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/send", nil)
	req3 = req3.WithContext(agentCtx)
	req3.Header.Set("X-Approval-Token", token)
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("replayed-token status = %d, want 403", w3.Code)
	}
}

// UAT step 4: accept a sent offer as a human principal, and the handler
// flips status to accepted without requiring an approval token.
func TestOfferHandler_Accept_SentOffer_NoTokenNeeded(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	db, h, wsID, offerID, humanID, agentID := setupSeededOfferWithLineItem(t, "EUR")

	token := signSendOfferToken(t, wsID, offerID)
	sendOfferAsAgentExpect200(t, h, wsID, agentID, offerID, token)

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/accept", nil)
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if status, ok := resp["status"].(string); !ok || status != "accepted" {
		t.Fatalf("expected status=accepted, got %v", resp["status"])
	}
	if resp["accepted_at"] == nil {
		t.Fatal("expected accepted_at populated")
	}

	var amountMinor int64
	var currency string
	if err := db.QueryRow(`SELECT amount_minor, currency FROM deal WHERE id=$1::uuid`, resp["deal_id"]).Scan(&amountMinor, &currency); err != nil {
		t.Fatalf("read deal: %v", err)
	}
	if amountMinor != 297500 || currency != "EUR" {
		t.Fatalf("expected deal sync to 297500 EUR, got amount_minor=%d currency=%s", amountMinor, currency)
	}
}

// UAT step 5: agent accept without a valid approval token is rejected with
// approval_required, mirroring sendOffer's tier-yellow gate.
func TestOfferHandler_Accept_AgentNoToken_403ApprovalRequired(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	_, h, wsID, offerID, _, agentID := setupSeededOfferWithLineItem(t, "EUR")

	token := signSendOfferToken(t, wsID, offerID)
	sendOfferAsAgentExpect200(t, h, wsID, agentID, offerID, token)

	agentCtx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: agentID, IsAgent: true})
	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/accept", nil)
	req = req.WithContext(agentCtx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if resp["code"] != "approval_required" {
		t.Fatalf("expected code=approval_required, got %v", resp["code"])
	}
}

// UAT step 5: after the rejected and accepted send calls, offer.sent is
// present exactly once for the original offer id.
func TestOfferHandler_Send_EmitsOfferSentExactlyOnce(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	db, h, wsID, offerID, _, agentID := setupSeededOfferWithLineItem(t, "EUR")
	agentCtx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID, UserID: agentID, IsAgent: true})

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/send", nil)
	req = req.WithContext(agentCtx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-token status = %d, want 403", w.Code)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='offer.sent' AND entity_id=$1::uuid`, offerID).Scan(&count); err != nil {
		t.Fatalf("count sent before approval: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero offer.sent rows before approved send, got %d", count)
	}

	token := signSendOfferToken(t, wsID, offerID)
	sendOfferAsAgentExpect200(t, h, wsID, agentID, offerID, token)

	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='offer.sent' AND entity_id=$1::uuid`, offerID).Scan(&count); err != nil {
		t.Fatalf("count sent after approval: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one offer.sent row, got %d", count)
	}
}

// UAT step 6: same-currency send freezes at the identity FX rate without any
// fx_rate row present for the workspace.
func TestOfferHandler_Send_SameCurrency_NoFXRowNeeded(t *testing.T) {
	_, h, wsID, offerID, humanID, _ := setupSeededOfferWithLineItem(t, "EUR")

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/send", nil)
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if rate, ok := resp["fx_rate_to_base"].(string); !ok || rate != "1.0000000000" {
		t.Fatalf("expected fx_rate_to_base=1.0000000000, got %v", resp["fx_rate_to_base"])
	}
}

// UAT step 7: cross-currency send without a stored rate returns 422
// fx_rate_unavailable.
func TestOfferHandler_Send_CrossCurrency_NoStoredRate_422(t *testing.T) {
	_, h, wsID, offerID, humanID, _ := setupSeededOfferWithLineItem(t, "USD")

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/send", nil)
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if resp["code"] != "fx_rate_unavailable" {
		t.Fatalf("expected code=fx_rate_unavailable, got %v", resp["code"])
	}
}

// UAT step 8: patching a sent offer still returns 409 offer_not_draft.
func TestOfferHandler_PatchAfterSend_Still409OfferNotDraft(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	_, h, wsID, offerID, humanID, agentID := setupSeededOfferWithLineItem(t, "EUR")

	token := signSendOfferToken(t, wsID, offerID)
	sendOfferAsAgentExpect200(t, h, wsID, agentID, offerID, token)

	body := map[string]any{"intro_text": "hello"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/offers/"+offerID, bytes.NewReader(b))
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if resp["code"] != "offer_not_draft" {
		t.Fatalf("expected code=offer_not_draft, got %v", resp["code"])
	}
}

// UAT step 9: regenerate a sent offer into a new draft revision and mark the
// prior row superseded.
func TestOfferHandler_Regenerate_SentOffer_NewDraftRevisionSupersedesePrior(t *testing.T) {
	t.Setenv("APPROVAL_TOKEN_SIGNING_SECRET", "merge-handler-it-secret")
	db, h, wsID, offerID, humanID, agentID := setupSeededOfferWithLineItem(t, "EUR")

	token := signSendOfferToken(t, wsID, offerID)
	sent := sendOfferAsAgentExpect200(t, h, wsID, agentID, offerID, token)
	if status, ok := sent["status"].(string); !ok || status != "sent" {
		t.Fatalf("expected sent status after send, got %v", sent["status"])
	}

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/regenerate", nil)
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	newID, _ := resp["id"].(string)
	if newID == "" || newID == offerID {
		t.Fatalf("expected a new offer id, got %q", newID)
	}
	if status, ok := resp["status"].(string); !ok || status != "draft" {
		t.Fatalf("expected new revision status=draft, got %v", resp["status"])
	}
	if rev, ok := resp["revision"].(float64); !ok || int64(rev) != 2 {
		t.Fatalf("expected new revision=2, got %v", resp["revision"])
	}

	gotPrior := getOffer(t, h, wsID, offerID, humanID)
	if status, ok := gotPrior["status"].(string); !ok || status != "superseded" {
		t.Fatalf("expected prior offer status=superseded, got %v", gotPrior["status"])
	}

	if items := offerLineItems(t, h, wsID, newID, humanID); len(items) != 1 {
		t.Fatalf("expected regenerated offer to keep one line item, got %d", len(items))
	}

	// entity_id is keyed on the prior (now-superseded) offer id, not the new
	// revision's — "offer.superseded" names the entity that was superseded
	// (mirrors OfferStore.Regenerate's merged convention, reconciled from
	// OP-T07's own surviving adapters-level test).
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM event_outbox WHERE topic='offer.superseded' AND entity_id=$1::uuid`, offerID).Scan(&count); err != nil {
		t.Fatalf("count superseded event: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one offer.superseded row for the prior offer, got %d", count)
	}
}

// UAT step 10: regenerate a still-draft offer -> 409 offer_not_sent.
func TestOfferHandler_Regenerate_DraftOffer_409OfferNotSent(t *testing.T) {
	_, h, wsID, offerID, humanID, _ := setupSeededOfferWithLineItem(t, "EUR")

	req := httptest.NewRequest(http.MethodPost, "/offers/"+offerID+"/regenerate", nil)
	req = withOfferWorkspace(req, wsID, humanID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if resp["code"] != "offer_not_sent" {
		t.Fatalf("expected code=offer_not_sent, got %v", resp["code"])
	}
}
