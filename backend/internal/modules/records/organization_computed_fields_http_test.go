//go:build integration

package records_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	activities "github.com/gradionhq/margince/backend/internal/modules/activities"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	orgtransport "github.com/gradionhq/margince/backend/internal/modules/organizations/transport"
	"github.com/gradionhq/margince/backend/internal/modules/records"
	relationships "github.com/gradionhq/margince/backend/internal/modules/relationships"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func computedFieldsHandlerForTest(db *sql.DB) http.Handler {
	return orgtransport.NewOrganizationHandler(
		orgAdapters.NewOrgStore(db),
		relationships.NewRelationshipStore(db),
		deals.NewDealStore(db),
		activities.NewActivityStore(db),
		records.NewRollupStore(db),
		&crmapprovals.DBVerifier{DB: db},
	)
}

func seedComputedFieldsRoleUser(t *testing.T, db *sql.DB, ws string, withComputedField bool) string {
	t.Helper()
	var userID string
	if err := db.QueryRow(
		`INSERT INTO app_user (workspace_id, email, display_name) VALUES ($1,$2,$3) RETURNING id`,
		ws,
		"cf-"+pgtest.Uniq()+"@example.com",
		"Computed Fields User",
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	perms := `{"organization":{"read":{"row_scope":"all"}}}`
	if withComputedField {
		perms = `{"organization":{"read":{"row_scope":"all"}},"computed_field":{"read":{"row_scope":"all"}}}`
	}
	var roleID string
	if err := db.QueryRow(
		`INSERT INTO role (workspace_id, key, is_system, permissions) VALUES ($1,$2,false,$3::jsonb) RETURNING id`,
		ws, "cf-"+pgtest.Uniq(), perms,
	).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO role_assignment (workspace_id, role_id, user_id) VALUES ($1,$2,$3)`,
		ws, roleID, userID,
	); err != nil {
		t.Fatalf("seed role_assignment: %v", err)
	}
	return userID
}

func withComputedFieldsPrincipal(r *http.Request, ws, userID string) *http.Request {
	return r.WithContext(crmctx.With(r.Context(), crmctx.Principal{TenantID: ws, UserID: userID}))
}

func createComputedFieldsOrg(t *testing.T, db *sql.DB, ws, name string) string {
	t.Helper()
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: ws, UserID: "human:test"})
	org, err := orgAdapters.NewOrgStore(db).Create(ctx, orgDomain.Organization{
		WorkspaceID: ws,
		DisplayName: name,
		Source:      "test",
		CapturedBy:  "human:test",
	}, nil)
	if err != nil {
		t.Fatalf("seed organization %s: %v", name, err)
	}
	return org.ID
}

func TestOrganization_Get_ComputedFieldsVisibleAndFloored(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	pipelineID, stageID := seedPipelineStage(t, db, ws)
	fx := "1.0000000000"
	amount := int64(212000)
	visibleUserID := seedComputedFieldsRoleUser(t, db, ws, true)

	orgWithDeal := createComputedFieldsOrg(t, db, ws, "Computed Fields Org")
	seedOpenDeal(t, db, ws, pipelineID, stageID, &orgWithDeal, &amount, &fx)

	orgNoDeal := createComputedFieldsOrg(t, db, ws, "Computed Fields Empty Org")
	h := computedFieldsHandlerForTest(db)

	req := httptest.NewRequest(http.MethodGet, "/organizations/"+orgWithDeal, nil)
	req = withComputedFieldsPrincipal(req, ws, visibleUserID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /organizations/{id}: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got orgDomain.Organization
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.ComputedFields) != 5 {
		t.Fatalf("computed_fields len=%d want 5", len(got.ComputedFields))
	}
	if got.ComputedFields[0].Key != "open_pipeline" {
		t.Fatalf("first computed field key=%q want open_pipeline", got.ComputedFields[0].Key)
	}
	var direct sql.NullInt64
	if err := db.QueryRow(
		`SELECT open_pipeline_minor_base FROM organization_open_pipeline_rollup WHERE organization_id = $1`,
		orgWithDeal,
	).Scan(&direct); err != nil {
		t.Fatalf("direct rollup read: %v", err)
	}
	if !direct.Valid {
		t.Fatal("direct rollup returned NULL for organization with open deal")
	}
	if got.ComputedFields[0].ValueMinor == nil || *got.ComputedFields[0].ValueMinor != direct.Int64 {
		t.Fatalf("open_pipeline.value_minor=%v want %d", got.ComputedFields[0].ValueMinor, direct.Int64)
	}
	for _, row := range got.ComputedFields[1:] {
		if row.Computable {
			t.Fatalf("%s should not be computable", row.Key)
		}
		if row.Reason == nil || *row.Reason != "not_yet_built" {
			t.Fatalf("%s reason=%v want not_yet_built", row.Key, row.Reason)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/organizations/"+orgNoDeal, nil)
	req = withComputedFieldsPrincipal(req, ws, visibleUserID)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /organizations/{id} empty org: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode empty-org response: %v", err)
	}
	if got.ComputedFields[0].ValueMinor == nil || *got.ComputedFields[0].ValueMinor != 0 {
		t.Fatalf("open_pipeline.value_minor=%v want 0", got.ComputedFields[0].ValueMinor)
	}
}

func TestOrganization_Get_ComputedFieldsHiddenWhenNotGranted(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	ws := pgtest.NewWorkspaceSQL(t, db)
	pgtest.SetRLS(t, db, ws)

	userID := seedComputedFieldsRoleUser(t, db, ws, false)
	orgID := createComputedFieldsOrg(t, db, ws, "Hidden Computed Fields Org")
	h := computedFieldsHandlerForTest(db)

	req := httptest.NewRequest(http.MethodGet, "/organizations/"+orgID, nil)
	req = withComputedFieldsPrincipal(req, ws, userID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /organizations/{id}: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if _, ok := raw["computed_fields"]; ok {
		t.Fatalf("computed_fields key present in response: %s", rec.Body.String())
	}
}
