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
	"github.com/riverqueue/river"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
	platformauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	platformconfig "github.com/gradionhq/margince/backend/internal/platform/config"
	"github.com/gradionhq/margince/backend/internal/platform/events"
	platformlogger "github.com/gradionhq/margince/backend/internal/platform/logger"
	crmmigrate "github.com/gradionhq/margince/backend/internal/shared/ports/migrate"
	"github.com/gradionhq/margince/backend/pkg/shared/ports/jurisdiction"
)

func main() {
	printJuris := flag.Bool("print-jurisdictions", false, "print linked jurisdiction codes and exit")
	migrateFlag := flag.Bool("migrate", false, "apply core + enabled-pack migrations to DATABASE_URL and exit")
	flag.Parse()
	if *printJuris {
		codes := jurisdiction.Codes()
		sort.Strings(codes)
		_, _ = fmt.Fprintln(os.Stdout, strings.Join(codes, "\n"))
		os.Exit(0)
	}

	cfg, err := platformconfig.Load()
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
		slog.SetDefault(slog.New(platformlogger.NewJSONHandler(os.Stdout)))
	}

	slog.Info(
		"margince server starting",
		"version", Version,
		"commit", Commit,
		"build_date", BuildDate,
		"jurisdictions_linked", jurisdiction.Codes(),
	)

	if err := serve(cfg); err != nil {
		log.Fatal(err)
	}
}

const serverReadHeaderTimeout = 10 * time.Second

func serve(cfg platformconfig.Config) error {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rdb, redisOK := events.NewRedisClient(cfg.RedisURL)
	if redisOK {
		events.StartOutboxRelay(ctx, db, rdb)
	}
	defer func() {
		if rdb != nil {
			_ = rdb.Close()
		}
	}()

	workers := river.NewWorkers()
	river.AddWorker(workers, crmgdpr.NewRetentionWorker(db))
	river.AddWorker(workers, crmapprovals.NewExpiryWorker(db))

	periodicJobs := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(24*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return crmgdpr.RetentionSweepArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: false},
		),
		river.NewPeriodicJob(
			river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return crmapprovals.ExpiryArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: false},
		),
	}

	riverClient, err := events.SetupRiver(ctx, db, workers, periodicJobs)
	if err != nil {
		return fmt.Errorf("setup river: %w", err)
	}

	go sampleGauges(ctx, db)

	mux := buildMux(ctx, db, cfg, riverClient)

	slog.Info("listening", "addr", cfg.Addr)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           platformauth.LogRequest(mux),
		ReadHeaderTimeout: serverReadHeaderTimeout,
	}
	return srv.ListenAndServe()
}
