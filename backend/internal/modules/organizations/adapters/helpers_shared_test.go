//go:build integration

// helpers_shared_test.go — shared test helpers for organizations/adapters integration tests.
// Duplicates the pattern from modules/deals/helpers_shared_test.go and
// modules/directory/helpers_shared_test.go — package adapters_test cannot share
// a _test.go file across a package boundary, so these are replicated here.
package adapters_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	orgDomain "github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

var (
	testSeq      int64
	testRunEpoch = time.Now().UnixNano()
)

// uniq returns a string unique across test binary invocations.
func uniq() string {
	return fmt.Sprintf("%d-%d", testRunEpoch, atomic.AddInt64(&testSeq, 1))
}

// openTestDB opens a *sql.DB from TEST_DATABASE_URL and registers a cleanup.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// sqlDB is an alias for openTestDB, matching the crmcore_test helper name used
// by tests originally in package crmcore_test.
func sqlDB(t *testing.T) *sql.DB { return openTestDB(t) }

// setRLS sets app.workspace_id on the connection so Postgres RLS applies.
func setRLS(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), "SET app.workspace_id = '"+wsID+"'")
	if err != nil {
		t.Fatal("setRLS:", err)
	}
}

// seedWorkspace inserts a workspace row, ignoring conflicts.
func seedWorkspace(t *testing.T, db *sql.DB, wsID string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace(id,name,slug,base_currency) VALUES($1,'t06orgtest',$2,'EUR') ON CONFLICT DO NOTHING`,
		wsID, "t06-org-"+wsID)
	if err != nil {
		t.Fatal("seed workspace:", err)
	}
}

// newWorkspaceSQL inserts a fresh workspace with an auto-generated UUID and
// returns the new ID. Used by tests that need a pristine, unique workspace per run.
func newWorkspaceSQL(t *testing.T, db *sql.DB) string {
	t.Helper()
	nonce := fmt.Sprintf("orgadapter-%d", time.Now().UnixNano())
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO workspace(name, slug, base_currency) VALUES ($1, $2, 'EUR') RETURNING id`,
		"ws-"+nonce, "slug-"+nonce).Scan(&id)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return id
}

// appCtx builds a principal-bearing context with a human user for store-layer
// audit writes (EntryFromPrincipal needs UserID+TenantID).
func appCtx(ws string) context.Context {
	return crmctx.With(context.Background(),
		crmctx.Principal{UserID: "human:store-rls-test", TenantID: ws})
}

// mergeTestCtx builds a principal context suitable for merge test scenarios.
func mergeTestCtx(ws string) context.Context {
	return crmctx.With(context.Background(),
		crmctx.Principal{UserID: "human:merge-test", TenantID: ws})
}

// assertNoRows fails the test if the query returns any rows.
func assertNoRows(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	var x int
	err := db.QueryRow(query, args...).Scan(&x)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no rows for %q, got x=%d err=%v", query, x, err)
	}
}

// fkIntoTable returns every (referencing_table→referencing_column) pair with a
// live FOREIGN KEY into table(id) — the DB-truth version of "grep migrations".
func fkIntoTable(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu ON tc.constraint_name = ccu.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name = $1`, table)
	if err != nil {
		t.Fatalf("fkIntoTable(%s): %v", table, err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var refTable, refCol string
		if err := rows.Scan(&refTable, &refCol); err != nil {
			t.Fatal(err)
		}
		out[refTable] = refCol
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("fkIntoTable(%s) rows: %v", table, err)
	}
	return out
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
