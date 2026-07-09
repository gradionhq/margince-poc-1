//go:build integration

package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/deals/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func createArchivableTestDeal(ctx context.Context, t *testing.T, dealStore *adapters.DealStore, pipelineID, stageID, name string) domain.Deal {
	t.Helper()
	d := domain.NewDeal(name, pipelineID, stageID, prov.Provenance{Source: "test", CapturedBy: "human:test"})
	d.WorkspaceID = dealTestWorkspaceID
	d, err := dealStore.Create(ctx, d, "", nil)
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}
	return d
}

func TestDealHandler_Archive_HappyPath200(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "archive")
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: "human:test"})
	dealStore := adapters.NewDealStore(db)

	d := createArchivableTestDeal(ctx, t, dealStore, pipelineID, stageID, "Handler Archivable Deal")

	h := dealHandlerForTest(db, dealStore)
	req := httptest.NewRequest(http.MethodDelete, "/deals/"+d.ID, nil)
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["archived_at"] == nil {
		t.Fatalf("want archived_at set in response, got nil")
	}
}

func TestDealHandler_Archive_NonExistentReturns404(t *testing.T) {
	db := openDealTestDB(t)
	_, _, _ = seedDealFixtures(t, db, "archive-404")
	h := dealHandlerForTest(db, adapters.NewDealStore(db))

	req := httptest.NewRequest(http.MethodDelete, "/deals/00000000-0000-0000-0000-000000000099", nil)
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if code, ok := resp["code"]; ok && code == "not_found" {
			return // expected app-level 404
		}
	}
	t.Fatal("expected app-level 404 with code='not_found', not plain http.NotFound")
}

func TestDealHandler_Archive_WrongWorkspaceReturns404(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "archive-workspace")
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: "human:test"})
	dealStore := adapters.NewDealStore(db)

	d := createArchivableTestDeal(ctx, t, dealStore, pipelineID, stageID, "Wrong Workspace Deal")

	h := dealHandlerForTest(db, dealStore)
	// Send request with a different workspace ID
	req := httptest.NewRequest(http.MethodDelete, "/deals/"+d.ID, nil)
	req = req.WithContext(crmctx.With(req.Context(), crmctx.Principal{TenantID: "00000000-0000-0000-0000-000000000099", UserID: "human:test"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestDealHandler_Archive_ArchivedDealExcludedFromList(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "archive-list")
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: "human:test"})
	dealStore := adapters.NewDealStore(db)

	d := createArchivableTestDeal(ctx, t, dealStore, pipelineID, stageID, "Archivable for List")

	h := dealHandlerForTest(db, dealStore)

	// Archive the deal
	archiveReq := httptest.NewRequest(http.MethodDelete, "/deals/"+d.ID, nil)
	archiveReq = withDealWorkspace(archiveReq)
	archiveW := httptest.NewRecorder()
	h.ServeHTTP(archiveW, archiveReq)
	if archiveW.Code != http.StatusOK {
		t.Fatalf("archive failed: status=%d, body=%s", archiveW.Code, archiveW.Body.String())
	}

	// List deals for this pipeline — archived deal should not appear
	listReq := httptest.NewRequest(http.MethodGet, "/deals?pipeline_id="+pipelineID, nil)
	listReq = withDealWorkspace(listReq)
	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list failed: status=%d", listW.Code)
	}
	var page struct {
		Data []domain.Deal `json:"data"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, deal := range page.Data {
		if deal.ID == d.ID {
			t.Fatalf("archived deal %s should not appear in default list", d.ID)
		}
	}
}
