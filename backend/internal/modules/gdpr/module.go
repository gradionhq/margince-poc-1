// Package crmgdpr implements GDPR engine: consent, retention, erasure and SAR.
// This module.go re-exports all public types and functions from the domain/,
// ports/, and adapters/ subdirectories so external callers see an unchanged API
// (WS-E-a structural migration).
package crmgdpr

import (
	"context"
	"database/sql"
	"time"

	gdpradapters "github.com/gradionhq/margince/backend/internal/modules/gdpr/adapters"
	gdprdomain "github.com/gradionhq/margince/backend/internal/modules/gdpr/domain"
	gdprports "github.com/gradionhq/margince/backend/internal/modules/gdpr/ports"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// ConsentState is the current consent status for a given (person, purpose) pair.
type ConsentState = gdprdomain.ConsentState

// Consent state constants.
const (
	Granted   = gdprdomain.Granted
	Withdrawn = gdprdomain.Withdrawn
	Unknown   = gdprdomain.Unknown
)

// SARPackage holds all data held about a subject for an Art. 15 Subject Access Request.
type SARPackage = gdprdomain.SARPackage

// Policy is a retention rule for an object type + optional category.
type Policy = gdprdomain.Policy

// RetentionSweepArgs is the payload for the nightly retention sweep job.
type RetentionSweepArgs = gdprdomain.RetentionSweepArgs

// ---------------------------------------------------------------------------
// Domain function wrappers
// ---------------------------------------------------------------------------

// OverAge reports whether lastActivity is more than retainDays×24h before asOf.
func OverAge(asOf time.Time, retainDays int, lastActivity time.Time) bool {
	return gdprdomain.OverAge(asOf, retainDays, lastActivity)
}

// ---------------------------------------------------------------------------
// Port type aliases
// ---------------------------------------------------------------------------

// ConsentRepository is the GDPR consent read seam for per-call consent checks.
type ConsentRepository = gdprports.ConsentRepository

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// RetentionWorker evaluates all retention policies across workspaces.
type RetentionWorker = gdpradapters.RetentionWorker

// ConsentRequest carries everything needed to record one consent signal.
type ConsentRequest = gdpradapters.ConsentRequest

// Querier is satisfied by *sql.DB, *sql.Tx, and *sql.Conn.
type Querier = gdpradapters.Querier

// ---------------------------------------------------------------------------
// Adapter constructor wrappers
// ---------------------------------------------------------------------------

// NewRetentionWorker returns a RetentionWorker backed by db.
func NewRetentionWorker(db *sql.DB) *RetentionWorker {
	return gdpradapters.NewRetentionWorker(db)
}

// NewConsentRepository returns a PostgreSQL-backed ConsentRepository.
//
//nolint:ireturn // seam returns the ConsentRepository interface by design
func NewConsentRepository(db *sql.DB) ConsentRepository {
	return gdpradapters.NewConsentRepository(db)
}

// ---------------------------------------------------------------------------
// Adapter function wrappers
// ---------------------------------------------------------------------------

// Erase irreversibly removes PII for a person from normalized tables.
func Erase(ctx context.Context, db *sql.DB, personID string) error {
	return gdpradapters.Erase(ctx, db, personID)
}

// Assemble gathers all data held about personID and writes a PII-free audit row.
func Assemble(ctx context.Context, db *sql.DB, personID string) (SARPackage, error) {
	return gdpradapters.Assemble(ctx, db, personID)
}

// Hash returns the SHA-256 hex digest of the email, lowercased and trimmed.
func Hash(email string) string {
	return gdpradapters.Hash(email)
}

// IsSuppressed reports whether email is in erasure_suppression for wsID.
func IsSuppressed(ctx context.Context, q Querier, wsID, email string) (bool, error) {
	return gdpradapters.IsSuppressed(ctx, q, wsID, email)
}

// Record records a consent signal in one atomic transaction.
func Record(ctx context.Context, db *sql.DB, w ConsentRequest) error {
	return gdpradapters.Record(ctx, db, w)
}

// SeedDefaults inserts the 5 default retention policies for workspaceID within tx.
func SeedDefaults(ctx context.Context, tx *sql.Tx, workspaceID string) error {
	return gdpradapters.SeedDefaults(ctx, tx, workspaceID)
}
