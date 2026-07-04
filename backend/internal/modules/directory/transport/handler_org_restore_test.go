//go:build integration

package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	crmcore "github.com/gradionhq/margince/backend/internal/modules/directory"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const orgRestoreHandlerTestWS = "00000000-0000-0000-0000-000000000052"

func seedOrgRestoreHandlerWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	orgHandlerSetRLS(t, db, orgRestoreHandlerTestWS)
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1, $2, $3, 'EUR') ON CONFLICT (id) DO NOTHING`,
		orgRestoreHandlerTestWS,
		"org-restore-handler-ws",
		"org-restore-handler-ws-"+time.Now().Format("20060102150405"),
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func withOrgRestoreWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: orgRestoreHandlerTestWS, UserID: "human:test"})
	return r.WithContext(ctx)
}

func TestOrganizationHandler_Restore_HappyPath200(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgRestoreHandlerWorkspace(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgRestoreHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)

	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgRestoreHandlerTestWS,
		DisplayName: "Handler Restorable Org",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if _, err := orgStore.Archive(ctx, org.ID, orgRestoreHandlerTestWS); err != nil {
		t.Fatalf("archive org: %v", err)
	}

	h := NewOrganizationHandler(orgStore, db)
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+org.ID+"/restore", nil)
	req = withOrgRestoreWorkspace(req)
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
	seedOrgRestoreHandlerWorkspace(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgRestoreHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)

	org, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgRestoreHandlerTestWS,
		DisplayName: "Already Live Handler Org",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	h := NewOrganizationHandler(orgStore, db)
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+org.ID+"/restore", nil)
	req = withOrgRestoreWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422, body=%s", w.Code, w.Body.String())
	}
}

func TestOrganizationHandler_Restore_RefusesMergedRecord(t *testing.T) {
	db := openDealTestDB(t)
	seedOrgRestoreHandlerWorkspace(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: orgRestoreHandlerTestWS, UserID: "human:test"})
	orgStore := crmcore.NewOrgStore(db)

	survivor, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgRestoreHandlerTestWS,
		DisplayName: "Survivor Handler Org",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create survivor org: %v", err)
	}
	merged, err := orgStore.Create(ctx, crmcore.Organization{
		WorkspaceID: orgRestoreHandlerTestWS,
		DisplayName: "Merged Handler Org",
		Source:      "test",
		CapturedBy:  "human:test",
	})
	if err != nil {
		t.Fatalf("create merged org: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE organization
		 SET archived_at = now(), merged_into_id = $1::uuid
		 WHERE id = $2::uuid AND workspace_id = $3::uuid`,
		survivor.ID, merged.ID, orgRestoreHandlerTestWS,
	); err != nil {
		t.Fatalf("seed merged organization state: %v", err)
	}

	h := NewOrganizationHandler(orgStore, db)
	req := httptest.NewRequest(http.MethodPost, "/organizations/"+merged.ID+"/restore", nil)
	req = withOrgRestoreWorkspace(req)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422, body=%s", w.Code, w.Body.String())
	}
}
