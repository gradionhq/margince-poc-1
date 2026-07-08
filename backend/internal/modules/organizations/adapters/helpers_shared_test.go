//go:build integration

// helpers_shared_test.go — organizations/adapters-specific integration-test
// fixtures. Generic Postgres test helpers (open DB, RLS, seed workspace, uniq,
// assertNoRows, fkIntoTable, appCtx) live in the Tier-0 shared/kernel/pgtest
// package; only module-specific fixtures remain here.
package adapters_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// mergeTestCtx builds a principal context suitable for merge test scenarios.
func mergeTestCtx(ws string) context.Context {
	return crmctx.With(context.Background(),
		crmctx.Principal{UserID: "human:merge-test", TenantID: ws})
}

// mkOrg creates an Organization via OrgStore.Create and returns it.
// Used by merge tests to seed loser/target orgs.
func mkOrg(ctx context.Context, t *testing.T, store *orgAdapters.OrgStore, ws, name string) orgDomain.Organization {
	t.Helper()
	created, err := store.Create(ctx, orgDomain.Organization{
		WorkspaceID: ws, DisplayName: name, Source: "api", CapturedBy: "human:t",
	})
	if err != nil {
		t.Fatalf("create org %s: %v", name, err)
	}
	return created
}

// mkDealForMergeTest seeds a pipeline, stage, and deal in the given workspace
// and returns the deal ID. Used by merge tests to prove deal relink.
func mkDealForMergeTest(t *testing.T, db *sql.DB, ws string) string {
	t.Helper()
	var pipelineID, stageID, dealID string
	db.QueryRow(`INSERT INTO pipeline (workspace_id, name, is_default) VALUES ($1,'MergeTestPipeline',true) RETURNING id`, ws).Scan(&pipelineID)
	db.QueryRow(`INSERT INTO stage (workspace_id, pipeline_id, name, position) VALUES ($1,$2,'Open',1) RETURNING id`, ws, pipelineID).Scan(&stageID)
	if err := db.QueryRow(`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, source, captured_by)
		VALUES ($1,'MergeTestDeal',$2,$3,'api','human:t') RETURNING id`, ws, pipelineID, stageID).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return dealID
}

// fixedStrengthClock is the pinned test clock for PO-F-3 / PO-N-ORGSTRENGTH
// tests — matches modules/directory/strength_test.go's TEST-DET-1 constant so
// any activity "N days before" anchors consistently across both packages.
var fixedStrengthClock = time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)

// _ ensures the ids package is imported for tests that use ids.New() via helpers.
var _ = ids.New
