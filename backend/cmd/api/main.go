// Command server is the Tier-3 composition root — the only place impls + packs
// are wired. WP1: HTTP server with REST CRUD routes for all core objects.
// EP03: session middleware, auth handlers, and passport handlers are wired here.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/platform/httpserver"
	obs "github.com/gradionhq/margince/backend/internal/shared/kernel/obs"
	crmmigrate "github.com/gradionhq/margince/backend/internal/shared/ports/migrate"
	"github.com/gradionhq/margince/backend/pkg/shared/ports/jurisdiction"
)

func main() {
	// Composition-switch probe: must run before any infra dial so the composition
	// test and UAT can invoke it with zero infrastructure.
	printJuris := flag.Bool("print-jurisdictions", false, "print linked jurisdiction codes and exit")
	// -migrate aggregates core + every ENABLED pack's migrations. "Enabled" =
	// "linked & registered": the existing compile-time switch is the gate, so a
	// `-tags nopacks` binary migrates core only while a DACH build migrates
	// core+DE. There is NO migration-specific build tag. Runs in this pre-infra
	// zone so a migrate run never dials river/redis/AI.
	migrateFlag := flag.Bool("migrate", false, "apply core + enabled-pack migrations to DATABASE_URL and exit")
	flag.Parse()
	if *printJuris {
		codes := jurisdiction.Codes()
		sort.Strings(codes)
		// Machine-readable probe output: one code per line on stdout, parsed by the
		// composition test. Fprintln (not slog) keeps the contract clean of log noise.
		_, _ = fmt.Fprintln(os.Stdout, strings.Join(codes, "\n"))
		os.Exit(0)
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if *migrateFlag {
		if err := crmmigrate.Run(context.Background(), cfg.DatabaseURL); err != nil {
			log.Fatalf("migrate: %v", err)
		}
		slog.Info("migrate: core + enabled packs applied", "jurisdictions", jurisdiction.Codes())
		os.Exit(0)
	}

	if cfg.LogFormat == "text" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
	} else {
		slog.SetDefault(slog.New(obs.NewJSONHandler(os.Stdout)))
	}

	slog.Info(
		"margince server starting",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
		"jurisdictions_linked", jurisdiction.Codes(),
	)

	// serve owns every infra dial behind a returned error, so all fatals collapse
	// to this one site — no log.Fatal stranded behind a defer (gocritic exitAfterDefer).
	if err := serve(cfg); err != nil {
		log.Fatal(err)
	}
}

// serverReadHeaderTimeout bounds how long a client may take to send request
// headers before the server gives up — the Slowloris guard gosec G114 asks for.
const serverReadHeaderTimeout = 10 * time.Second

// serve dials infrastructure (db, River, Redis, AI runtime), wires the route mux,
// and blocks in ListenAndServe. It returns the first fatal error rather than
// calling log.Fatal itself, so its deferred cleanup always runs.
func serve(cfg Config) error {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rdb, redisOK := newRedisClient(cfg)
	if redisOK {
		startOutboxRelay(ctx, db, rdb)
	}
	defer func() {
		if rdb != nil {
			_ = rdb.Close()
		}
	}()

	riverClient, err := setupRiver(ctx, db, rdb, cfg)
	if err != nil {
		return fmt.Errorf("setup river: %w", err)
	}

	go sampleGauges(ctx, db)

	mux := buildMux(ctx, db, cfg, riverClient)

	slog.Info("listening", "addr", cfg.Addr)
	// An explicit Server with a ReadHeaderTimeout bounds slow-header (Slowloris)
	// clients; the bare http.ListenAndServe has none (gosec G114).
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpserver.LogRequest(mux),
		ReadHeaderTimeout: serverReadHeaderTimeout,
	}
	return srv.ListenAndServe()
}
