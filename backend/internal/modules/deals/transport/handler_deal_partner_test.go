//go:build integration

package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const handlerDealPartnerTestWorkspaceID = "00000000-0000-0000-0000-000000000007"

func seedHandlerDealPartnerFixtures(t *testing.T, tag string) (pipelineID, stageID, orgAID, orgBID string) {
	t.Helper()
	db := openDealTestDB(t)
	tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	if _, err := db.Exec(`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1,'t15-hd-ws',$2,'EUR')
		ON CONFLICT (id) DO NOTHING`, handlerDealPartnerTestWorkspaceID, "t15-hd-ws-"+tag); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := db.Exec(`SELECT set_config('app.workspace_id', $1, false)`, handlerDealPartnerTestWorkspaceID); err != nil {
		t.Fatalf("set rls: %v", err)
	}
	var p string
	if err := db.QueryRow(`INSERT INTO pipeline (id, workspace_id, name) VALUES (uuidv7(), $1, $2) RETURNING id`,
		handlerDealPartnerTestWorkspaceID, "P-"+tag).Scan(&p); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	var s string
	if err := db.QueryRow(`INSERT INTO stage (id, workspace_id, pipeline_id, name, position) VALUES (uuidv7(), $1, $2, $3, 1) RETURNING id`,
		handlerDealPartnerTestWorkspaceID, p, "S-"+tag).Scan(&s); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	var orgA, orgB string
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, classification, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'partner', 'test', 'human:test') RETURNING id`, handlerDealPartnerTestWorkspaceID, "OrgA-"+tag).Scan(&orgA); err != nil {
		t.Fatalf("seed org A: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO organization (id, workspace_id, name, classification, source, captured_by)
		VALUES (uuidv7(), $1, $2, 'partner', 'test', 'human:test') RETURNING id`, handlerDealPartnerTestWorkspaceID, "OrgB-"+tag).Scan(&orgB); err != nil {
		t.Fatalf("seed org B: %v", err)
	}
	for _, orgID := range []string{orgA, orgB} {
		if _, err := db.Exec(`INSERT INTO partner (id, workspace_id, organization_id, cert_status, source, captured_by)
			VALUES (uuidv7(), $1, $2, 'applied', 'test', 'human:test')`, handlerDealPartnerTestWorkspaceID, orgID); err != nil {
			t.Fatalf("seed partner row for org %s: %v", orgID, err)
		}
	}
	return p, s, orgA, orgB
}

func withHandlerDealPartnerWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: handlerDealPartnerTestWorkspaceID, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestDealHandler_CreateUpdateAndList_RoundTripPartnerOrgID(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, orgA, orgB := seedHandlerDealPartnerFixtures(t, "roundtrip")
	h := dealHandlerForTest(db, adapters.NewDealStore(db))

	createBody := map[string]any{
		"name": "Deal via partner", "pipeline_id": pipelineID, "stage_id": stageID,
		"partner_org_id": orgA, "source": "test", "captured_by": "human:test",
	}
	cb, _ := json.Marshal(createBody)
	createReq := withHandlerDealPartnerWorkspace(httptest.NewRequest(http.MethodPost, "/deals", bytes.NewReader(cb)))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createW.Code, createW.Body.String())
	}
	var created domain.Deal
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created deal: %v", err)
	}
	if created.PartnerOrgID == nil || *created.PartnerOrgID != orgA {
		t.Fatalf("created deal partner_org_id = %v, want %s", created.PartnerOrgID, orgA)
	}

	updateBody := map[string]any{"partner_org_id": orgB}
	ub, _ := json.Marshal(updateBody)
	updateReq := withHandlerDealPartnerWorkspace(httptest.NewRequest(http.MethodPatch, "/deals/"+created.ID, bytes.NewReader(ub)))
	updateW := httptest.NewRecorder()
	h.ServeHTTP(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateW.Code, updateW.Body.String())
	}
	var updated domain.Deal
	if err := json.Unmarshal(updateW.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated deal: %v", err)
	}
	if updated.PartnerOrgID == nil || *updated.PartnerOrgID != orgB {
		t.Fatalf("updated deal partner_org_id = %v, want %s", updated.PartnerOrgID, orgB)
	}

	listReq := withHandlerDealPartnerWorkspace(httptest.NewRequest(http.MethodGet, "/deals?partner_org_id="+orgB, nil))
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listW.Code, listW.Body.String())
	}
	var page struct {
		Data []domain.Deal `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, d := range page.Data {
		if d.ID == created.ID {
			found = true
			if d.PartnerOrgID == nil || *d.PartnerOrgID != orgB {
				t.Fatalf("listDeals?partner_org_id=%s returned deal with partner_org_id = %v, want %s", orgB, d.PartnerOrgID, orgB)
			}
		}
	}
	if !found {
		t.Fatalf("listDeals?partner_org_id=%s did not return the created deal", orgB)
	}
}
