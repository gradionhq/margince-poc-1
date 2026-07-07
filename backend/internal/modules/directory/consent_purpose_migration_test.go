//go:build integration

// consent_purpose_migration_test.go proves migration 000070_ws_c_conformance's
// consent_purpose remap (D2 step 4) survives the consent_event append-only
// trigger. It runs on its own throwaway scratch database (created/dropped here,
// distinct from TEST_DATABASE_URL's own DB) because it must apply migrations
// only up to 000070's immediate predecessor, seed data against the OLD global
// consent_purpose shape, then apply 000070 and assert the remap — a scenario
// the shared, already-fully-migrated TEST_DATABASE_URL database cannot express.
package crmcore_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // registers the file:// migration source
	_ "github.com/lib/pq"                                // registers the "postgres" database/sql driver
)

// wsCConformanceVersion is this task's migration version (000070). Re-confirmed
// at implementation time per the plan's D1 note: `ls backend/migrations/*.up.sql
// | sort | tail -1` showed 000068 with 000069 reserved (unmerged GH-209) —
// 000070 is the next free number.
const wsCConformanceVersion = 70

// migrationsDir resolves backend/migrations relative to this source file (not
// the test binary's working directory), so it is correct regardless of how `go
// test` is invoked.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	return dir
}

// priorMigrationVersion returns the highest migration version strictly below
// before found in dir — i.e. 000070's immediate predecessor on this branch
// (000069 is reserved for GH-209's unmerged branch and must never be targeted
// explicitly; computing this dynamically means it's never hardcoded here).
func priorMigrationVersion(t *testing.T, dir string, before uint) uint {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	re := regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)
	var best uint
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			continue
		}
		if uint(v) < before && uint(v) > best {
			best = uint(v)
		}
	}
	if best == 0 {
		t.Fatalf("no migration version found before %d in %s", before, dir)
	}
	return best
}

// adminBaseURL parses TEST_DATABASE_URL so the scratch database can be created
// on the same Postgres server/credentials the rest of the integration suite uses.
func adminBaseURL(t *testing.T) *url.URL {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	return u
}

func TestConsentEventRemapSurvivesImmutabilityTrigger(t *testing.T) {
	base := adminBaseURL(t)

	adminURL := *base
	adminURL.Path = "/postgres"
	adminDB, err := sql.Open("postgres", adminURL.String())
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}

	scratchName := fmt.Sprintf("gh81_consent_remap_%d", os.Getpid())
	if _, err := adminDB.Exec(`DROP DATABASE IF EXISTS ` + scratchName + ` WITH (FORCE)`); err != nil {
		t.Fatalf("drop stale scratch db: %v", err)
	}
	if _, err := adminDB.Exec(`CREATE DATABASE ` + scratchName); err != nil {
		t.Fatalf("create scratch db: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec(`DROP DATABASE IF EXISTS ` + scratchName + ` WITH (FORCE)`)
		_ = adminDB.Close()
	})

	scratchURL := *base
	scratchURL.Path = "/" + scratchName
	scratchDB, err := sql.Open("postgres", scratchURL.String())
	if err != nil {
		t.Fatalf("open scratch db: %v", err)
	}
	t.Cleanup(func() { _ = scratchDB.Close() })

	dir := migrationsDir(t)
	priorVersion := priorMigrationVersion(t, dir, wsCConformanceVersion)

	driver, err := postgres.WithInstance(scratchDB, &postgres.Config{})
	if err != nil {
		t.Fatalf("postgres driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+dir, "postgres", driver)
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	if err := m.Migrate(uint(priorVersion)); err != nil {
		t.Fatalf("migrate to version %d: %v", priorVersion, err)
	}

	// Seed a pre-existing workspace + person + consent_event against the OLD
	// global consent_purpose shape. A fresh/empty DB has zero consent_event
	// rows, so a test that migrated an empty DB would pass even without the
	// disable/enable trigger bracket (D2 step 4) — this fixture exists
	// specifically to catch that regression.
	ctx := context.Background()
	var wsID, personID, purposeID string
	if err := scratchDB.QueryRowContext(
		ctx,
		`INSERT INTO workspace(name,slug,base_currency) VALUES('remap-ws','remap-ws','USD') RETURNING id`,
	).Scan(&wsID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := scratchDB.ExecContext(ctx, `SELECT set_config('app.workspace_id',$1,false)`, wsID); err != nil {
		t.Fatalf("set guc: %v", err)
	}
	if err := scratchDB.QueryRowContext(ctx,
		`INSERT INTO person(workspace_id,full_name,source,captured_by,version) VALUES($1,'Remap Person','test','test',1) RETURNING id`,
		wsID).Scan(&personID); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	if err := scratchDB.QueryRowContext(ctx,
		`SELECT id FROM consent_purpose WHERE name='marketing_email'`).Scan(&purposeID); err != nil {
		t.Fatalf("resolve old global purpose: %v", err)
	}
	var eventID string
	if err := scratchDB.QueryRowContext(ctx, `
		INSERT INTO consent_event(workspace_id,person_id,purpose_id,event_state,policy_wording,policy_version,source)
		VALUES($1,$2,$3,'granted','wording','v1','test') RETURNING id`,
		wsID, personID, purposeID).Scan(&eventID); err != nil {
		t.Fatalf("seed consent_event: %v", err)
	}

	var preCount int
	if err := scratchDB.QueryRowContext(ctx, `SELECT count(*) FROM consent_event`).Scan(&preCount); err != nil {
		t.Fatalf("count pre-migration consent_event: %v", err)
	}
	if preCount != 1 {
		t.Fatalf("pre-migration consent_event count = %d, want 1 (fixture setup bug)", preCount)
	}

	if err := m.Migrate(wsCConformanceVersion); err != nil {
		t.Fatalf("migrate to %d: %v", wsCConformanceVersion, err)
	}

	// The seeded event's purpose_id must now point at the workspace-scoped row
	// with the same key, with no trigger-violation error (D2 step 4).
	var gotWorkspaceID, gotKey string
	if err := scratchDB.QueryRowContext(ctx, `
		SELECT cp.workspace_id, cp.key
		FROM consent_event ce JOIN consent_purpose cp ON cp.id = ce.purpose_id
		WHERE ce.id = $1`, eventID).Scan(&gotWorkspaceID, &gotKey); err != nil {
		t.Fatalf("read remapped consent_event: %v", err)
	}
	if gotWorkspaceID != wsID {
		t.Errorf("consent_event.purpose_id workspace_id = %s, want %s", gotWorkspaceID, wsID)
	}
	if gotKey != "marketing_email" {
		t.Errorf("consent_event.purpose_id key = %s, want marketing_email", gotKey)
	}

	// The migrated workspace ends up with exactly 4 purposes (the backfill
	// clone, D2 step 3) — distinct ids, same key set as the old global rows.
	var purposeCount int
	if err := scratchDB.QueryRowContext(ctx,
		`SELECT count(*) FROM consent_purpose WHERE workspace_id=$1`, wsID).Scan(&purposeCount); err != nil {
		t.Fatalf("count workspace purposes: %v", err)
	}
	if purposeCount != 4 {
		t.Errorf("workspace %s: consent_purpose count = %d, want 4", wsID, purposeCount)
	}

	// The append-only trigger must be re-enabled (not left disabled) after the
	// migration — an UPDATE on consent_event must still raise.
	if _, err := scratchDB.ExecContext(ctx, `UPDATE consent_event SET source='mutated' WHERE id=$1`, eventID); err == nil {
		t.Fatal("consent_event UPDATE should raise (append-only trigger must be re-enabled after the migration)")
	}
}
