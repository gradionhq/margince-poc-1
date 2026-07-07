// Package leads is the leads domain module: thin prospect records (data-model §8, ADR-0008).
// This module.go re-exports all public types and functions from the
// domain/ and adapters/ subdirectories so external callers see an
// unchanged API (WS-E-a structural migration).
package leads

import (
	"context"
	"database/sql"

	"github.com/gradionhq/margince/backend/internal/modules/leads/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/leads/domain"
	prov "github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// Lead is a thin, segregated prospect record (data-model §8, ADR-0008).
type Lead = domain.Lead

// Lead status constants.
const (
	StatusNew          = domain.StatusNew
	StatusWorking      = domain.StatusWorking
	StatusPromoted     = domain.StatusPromoted
	StatusDisqualified = domain.StatusDisqualified
)

// ---------------------------------------------------------------------------
// Domain function wrappers
// ---------------------------------------------------------------------------

// NewLead returns a Lead with a fresh ID, new status, version 1, and copied provenance.
func NewLead(p prov.Provenance) Lead {
	return domain.NewLead(p)
}

// ---------------------------------------------------------------------------
// Adapter type aliases
// ---------------------------------------------------------------------------

// LeadStore manages lead rows including the promote transaction.
type LeadStore = adapters.LeadStore

// ErrLeadEmailDuplicate is returned by LeadStore.Create when a non-archived lead
// with the same (workspace_id, lower(email)) already exists.
type ErrLeadEmailDuplicate = adapters.ErrLeadEmailDuplicate

// ---------------------------------------------------------------------------
// Adapter constructor wrappers
// ---------------------------------------------------------------------------

// NewLeadStore returns a LeadStore backed by db.
func NewLeadStore(db *sql.DB) *LeadStore {
	return adapters.NewLeadStore(db)
}

// ---------------------------------------------------------------------------
// Module
// ---------------------------------------------------------------------------

// Module is the leads module's dependency-injection handle.
type Module struct {
	LeadStore *adapters.LeadStore
}

// New returns a new leads Module wired to db.
func New(ctx context.Context, db *sql.DB) *Module {
	return &Module{
		LeadStore: adapters.NewLeadStore(db),
	}
}
