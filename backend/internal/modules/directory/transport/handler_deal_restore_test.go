//go:build integration

package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func TestDealHandler_Restore_HappyPath200(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "restore")
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: "human:test"})
	dealStore := crmcore.NewDealStore(db)

	d := crmcore.NewDeal("Handler Restorable Deal", pipelineID, stageID, prov.Provenance{
		Source: "test", CapturedBy: "human:test",
	})
	d.WorkspaceID = dealTestWorkspaceID
	d, err := dealStore.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}
	if _, err := dealStore.Archive(ctx, d.ID, dealTestWorkspaceID); err != nil {
		t.Fatalf("archive deal: %v", err)
	}

	h := NewDealHandler(dealStore, crmcore.NewRelationshipStore(db), db)
	req := httptest.NewRequest(http.MethodPost, "/deals/"+d.ID+"/restore", nil)
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
	if resp["archived_at"] != nil {
		t.Fatalf("want archived_at null in response, got %v", resp["archived_at"])
	}
}

func TestDealHandler_Restore_LiveRecordReturns422(t *testing.T) {
	db := openDealTestDB(t)
	pipelineID, stageID, _ := seedDealFixtures(t, db, "restore-live")
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: dealTestWorkspaceID, UserID: "human:test"})
	dealStore := crmcore.NewDealStore(db)

	d := crmcore.NewDeal("Already Live Handler Deal", pipelineID, stageID, prov.Provenance{
		Source: "test", CapturedBy: "human:test",
	})
	d.WorkspaceID = dealTestWorkspaceID
	d, err := dealStore.Create(ctx, d, "")
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}

	h := NewDealHandler(dealStore, crmcore.NewRelationshipStore(db), db)
	req := httptest.NewRequest(http.MethodPost, "/deals/"+d.ID+"/restore", nil)
	req = withDealWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422, body=%s", w.Code, w.Body.String())
	}
}
