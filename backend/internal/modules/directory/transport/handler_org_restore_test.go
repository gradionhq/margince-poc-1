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
)

func TestOrganizationHandler_Restore_HappyPath200(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)

	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS,
		DisplayName: "Handler Restorable Org",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := orgStore.Archive(ctx, org.ID, orgHandlerTestWS); err != nil {
		t.Fatalf("archive org: %v", err)
	}

	h := NewOrganizationHandler(orgStore)
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+org.ID+"/restore", nil)
	req = withOrgWorkspace(req)
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

func TestOrganizationHandler_Restore_LiveRecordReturns422(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgHandlerWorkspace(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)

	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgHandlerTestWS,
		DisplayName: "Already Live Handler Org",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	h := NewOrganizationHandler(orgStore)
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+org.ID+"/restore", nil)
	req = withOrgWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422, body=%s", w.Code, w.Body.String())
	}
}
