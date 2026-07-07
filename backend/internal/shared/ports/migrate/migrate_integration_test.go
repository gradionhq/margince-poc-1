//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/pkg/shared/ports/jurisdiction"
)

// openTestDB mirrors crm/crm-capture/integration_test.go::openTestDB: skip
// cleanly without TEST_DATABASE_URL (set by `make test-db-up`).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// coreMigrationsDir resolves backend/migrations relative to this file (the test
// runs with CWD = the package dir, so derive an absolute path from runtime.Caller).
func coreMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../backend/internal/shared/ports/migrate/<file> → up five → repo root →
	// backend/migrations. (1c restructure, task-3-brief.md: this package moved one
	// level deeper than its original internal/migrate/ home — up three → up five.
	// task-5-brief.md: migrations relocated from infra/ to backend/ — depth unchanged.)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "backend", "migrations")
}

// fakePack is a synthetic jurisdiction.Pack used to exercise the runner WITHOUT
// importing any real pack module (that would be a dependency cycle: crm → crm-de
// → crm). Its code "de" is allowed in core by the fitness gate (the scanned
// scanned pattern is country names, not a bare "de"); its migrations are an
// in-memory, country-neutral probe table — no pack-specific table literal here.
type fakePack struct{ code string }

func (p fakePack) Code() string                                { return p.code }
func (fakePack) Fiscal() jurisdiction.FiscalFormatter          { return nil }
func (fakePack) Retention() jurisdiction.RetentionPolicy       { return nil }
func (fakePack) Conformity() jurisdiction.ConformityRegime     { return nil }
func (fakePack) TrustArtifacts() jurisdiction.TrustArtifactSet { return nil }
func (fakePack) ExportProfiles() []jurisdiction.ExportProfile  { return nil }
func (fakePack) Migrations() fs.FS {
	return fstest.MapFS{
		"000001_probe.up.sql":   {Data: []byte("CREATE TABLE IF NOT EXISTS migrate_probe (id int);")},
		"000001_probe.down.sql": {Data: []byte("DROP TABLE IF EXISTS migrate_probe;")},
	}
}

func regclassExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var reg sql.NullString
	if err := db.QueryRow(`SELECT to_regclass($1)`, name).Scan(&reg); err != nil {
		t.Fatalf("to_regclass(%q): %v", name, err)
	}
	return reg.Valid
}

// TestRunWithAggregatesEnabledSet proves the GENERIC, jurisdiction-agnostic
// acceptance: with one enabled pack the runner creates a distinct per-pack
// tracking table after core; with no packs it creates none. Country-specific
// proof (the pack table's presence + RLS) lives in crm-de (the exempt module).
func TestRunWithAggregatesEnabledSet(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	coreDir := coreMigrationsDir(t)
	dsn := os.Getenv("TEST_DATABASE_URL")

	// Self-clean: this test owns schema_migrations_de + the probe table in public.
	clean := func() {
		db.ExecContext(ctx, `DROP TABLE IF EXISTS schema_migrations_de`)
		db.ExecContext(ctx, `DROP TABLE IF EXISTS migrate_probe`)
	}
	clean()
	t.Cleanup(clean)

	// Enabled-set phase: core (already applied → ErrNoChange) + one pack.
	if err := RunWith(ctx, dsn, coreDir, []jurisdiction.Pack{fakePack{code: "de"}}); err != nil {
		t.Fatalf("RunWith with one pack: %v", err)
	}
	if !regclassExists(t, db, "workspace") {
		t.Error("core table workspace missing after run")
	}
	if !regclassExists(t, db, "schema_migrations") {
		t.Error("core tracking table schema_migrations missing")
	}
	if !regclassExists(t, db, "schema_migrations_de") {
		t.Error("per-pack tracking table schema_migrations_de missing after enabled-set run")
	}
	if !regclassExists(t, db, "migrate_probe") {
		t.Error("pack migration did not apply (migrate_probe missing)")
	}
	// The per-pack table must be DISTINCT from core's default tracking table.
	if packTable("de") == "schema_migrations" {
		t.Error("per-pack table collides with core schema_migrations")
	}

	// No-pack phase: reset, run core-only → no per-pack tracking table appears.
	clean()
	if err := RunWith(ctx, dsn, coreDir, nil); err != nil {
		t.Fatalf("RunWith with no packs: %v", err)
	}
	if !regclassExists(t, db, "workspace") {
		t.Error("core table workspace missing in no-pack phase")
	}
	if regclassExists(t, db, "schema_migrations_de") {
		t.Error("no-pack phase must not create a per-pack tracking table")
	}
}
