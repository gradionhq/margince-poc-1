package events

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
)

// SetupRiver applies the River schema migrations to db, registers the supplied
// workers and periodic jobs, and starts the worker pool. Returns the River
// client so callers can insert jobs. The worker pool runs until ctx is
// cancelled.
//
// Domain-specific worker construction (e.g. gdpr.NewRetentionWorker,
// approvals.NewExpiryWorker) is the caller's responsibility — pass a
// pre-populated *river.Workers and a slice of *river.PeriodicJob so this
// function stays free of domain-module imports.
func SetupRiver(ctx context.Context, db *sql.DB, workers *river.Workers, periodicJobs []*river.PeriodicJob) (*river.Client[*sql.Tx], error) {
	driver := riverdatabasesql.New(db)

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

	riverClient, err := river.NewClient(driver, &river.Config{
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		PeriodicJobs: periodicJobs,
	})
	if err != nil {
		return nil, fmt.Errorf("river.NewClient: %w", err)
	}

	if err := riverClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("river start: %w", err)
	}

	return riverClient, nil
}
