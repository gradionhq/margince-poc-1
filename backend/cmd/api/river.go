package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
)

// setupRiver applies the River schema migrations to db, registers all workers,
// and starts the worker pool. Returns the River client so callers can insert jobs.
// The worker pool runs until ctx is cancelled.
//
// This is the pruned platform surface (skeleton harvest): only the workers/jobs
// backed by kept platform modules (crm-gdpr retention, crm-approvals expiry)
// survive. The frozen poc's capture/export/import/sync/backfill/bulk-operation/
// lead-score workers are not registered here — their modules were pruned.
func setupRiver(ctx context.Context, db *sql.DB, _ *redis.Client, _ Config) (*river.Client[*sql.Tx], error) {
	driver := riverdatabasesql.New(db)

	// Apply River's own schema migrations (river_job, river_leader, etc.).
	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		return nil, fmt.Errorf("rivermigrate.New: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return nil, fmt.Errorf("river migrate: %w", err)
	}
	if len(res.Versions) > 0 {
		log.Printf("river migration applied: %d version(s)", len(res.Versions))
	} else {
		log.Printf("river migration applied: schema already current")
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, crmgdpr.NewRetentionWorker(db))
	river.AddWorker(workers, crmapprovals.NewExpiryWorker(db))

	retentionSweepJob := river.NewPeriodicJob(
		river.PeriodicInterval(24*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return crmgdpr.RetentionSweepArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: false},
	)

	approvalExpiryJob := river.NewPeriodicJob(
		river.PeriodicInterval(1*time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return crmapprovals.ExpiryArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: false},
	)

	riverClient, err := river.NewClient(driver, &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		PeriodicJobs: []*river.PeriodicJob{retentionSweepJob, approvalExpiryJob},
	})
	if err != nil {
		return nil, fmt.Errorf("river.NewClient: %w", err)
	}

	if err := riverClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("river start: %w", err)
	}

	return riverClient, nil
}
