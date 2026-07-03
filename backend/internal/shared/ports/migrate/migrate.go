// Package migrate is the jurisdiction-agnostic migration aggregator. It applies
// core's migrations into the default schema_migrations table, then each ENABLED
// pack's own migrations into a per-pack tracking table (schema_migrations_<code>),
// applied after core.
//
// "Enabled" = "registered". The existing compile-time composition switch
// (cmd/server's !nopacks/nopacks blank-import set, ADR-0042) decides which packs
// link and self-register; this runner just iterates jurisdiction.Codes()
// generically. There is NO migration-specific build tag or env flag, and NO
// country identifier anywhere in this package — the per-pack table name is
// derived from Pack.Code() at runtime.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // registers the file:// migration source used by applyCore
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq" // registers the "postgres" database/sql driver

	"github.com/gradionhq/margince/backend/pkg/jurisdiction"
)

// packTable is the per-pack version-tracking table for a jurisdiction code,
// kept distinct from core's default schema_migrations.
func packTable(code string) string { return "schema_migrations_" + code }

// Run applies core migrations then every enabled pack's migrations to dbURL.
// coreDir is the conventional repo location of core's migrations.
func Run(ctx context.Context, dbURL string) error {
	return RunWith(ctx, dbURL, "backend/migrations", packsFromRegistry())
}

// packsFromRegistry resolves the registered (= compile-time-enabled) packs in a
// stable order, so a migrate run is deterministic.
func packsFromRegistry() []jurisdiction.Pack {
	codes := jurisdiction.Codes()
	sort.Strings(codes)
	out := make([]jurisdiction.Pack, 0, len(codes))
	for _, c := range codes {
		if p, ok := jurisdiction.For(c); ok {
			out = append(out, p)
		}
	}
	return out
}

// RunWith applies core (from coreDir, into schema_migrations) then each pack's
// embedded migrations (into schema_migrations_<code>). Packs with no migrations
// FS are skipped. ErrNoChange is treated as success at every stage. Exported so
// a pack's own integration test (a different module) can drive it with an
// explicit pack set and coreDir; production callers use Run.
func RunWith(ctx context.Context, dbURL, coreDir string, packs []jurisdiction.Pack) error {
	dsn, err := withLockTimeout(dbURL)
	if err != nil {
		return fmt.Errorf("migrate: build dsn: %w", err)
	}

	if err := applyCore(ctx, dsn, coreDir); err != nil {
		return fmt.Errorf("migrate core: %w", err)
	}

	for _, p := range packs {
		mfs := p.Migrations()
		if mfs == nil {
			continue
		}
		if err := applyPack(ctx, dsn, mfs, packTable(p.Code())); err != nil {
			return fmt.Errorf("migrate pack %q: %w", p.Code(), err)
		}
	}
	return nil
}

// withLockTimeout appends a server-side lock_timeout to the DSN. Setting it as a
// connection option (vs. SET on a pooled *sql.DB) guarantees it binds to every
// connection golang-migrate later checks out, not just the one that ran the SET
// (ADR-0017 §41).
func withLockTimeout(dbURL string) (string, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("options", "-c lock_timeout=2s")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// applyCore runs the core migrations from a file:// source into the default
// schema_migrations table.
func applyCore(ctx context.Context, dsn, coreDir string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("postgres driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+coreDir, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	return up(m)
}

// applyPack runs a pack's embedded migrations (iofs source) into its own
// per-pack tracking table.
func applyPack(ctx context.Context, dsn string, mfs fs.FS, table string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	src, err := iofs.New(mfs, ".")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{MigrationsTable: table})
	if err != nil {
		return fmt.Errorf("postgres driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	return up(m)
}

// up runs m.Up(), treating ErrNoChange as success.
func up(m *migrate.Migrate) error {
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
