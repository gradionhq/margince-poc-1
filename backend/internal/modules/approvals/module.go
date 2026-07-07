// Package crmapprovals owns the approval-inbox for staged 🟡 MCP tool actions.
// This module.go re-exports all public types and functions from the domain/,
// ports/, adapters/, and app/ subdirectories so external callers see an
// unchanged API (WS-E-a structural migration).
package crmapprovals

import (
	"context"
	"database/sql"
	"encoding/json"

	approvalsadapters "github.com/gradionhq/margince/backend/internal/modules/approvals/adapters"
	approvalsapp "github.com/gradionhq/margince/backend/internal/modules/approvals/app"
	approvalsdomain "github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
	approvalsports "github.com/gradionhq/margince/backend/internal/modules/approvals/ports"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// ---------------------------------------------------------------------------
// Domain type aliases
// ---------------------------------------------------------------------------

// Status is the lifecycle state of an approval_item row.
type Status = approvalsdomain.Status

// The approval_item lifecycle states.
const (
	StatusPending  = approvalsdomain.StatusPending
	StatusApproved = approvalsdomain.StatusApproved
	StatusRejected = approvalsdomain.StatusRejected
	StatusModified = approvalsdomain.StatusModified
	StatusExpired  = approvalsdomain.StatusExpired
)

// Item is one approval_item row.
type Item = approvalsdomain.Item

// ExpiryArgs is the River job payload for the approval expiry sweep.
type ExpiryArgs = approvalsdomain.ExpiryArgs

// TokenClaims is the ApprovalToken claim set.
type TokenClaims = approvalsdomain.TokenClaims

// EgressFlag marks a field in a send body as sensitive per §4.4.
type EgressFlag = approvalsdomain.EgressFlag

// ---------------------------------------------------------------------------
// Port type aliases
// ---------------------------------------------------------------------------

// DBExec is satisfied by *sql.Tx and *sql.DB.
type DBExec = approvalsports.DBExec

// Repository is the persistence seam for approval_item rows.
type Repository = approvalsports.Repository

// PageRepository is Repository plus the cursor-paged inbox projection.
type PageRepository = approvalsports.PageRepository

// EventEmitter is a narrow seam for writing outbox events inside an open tx.
type EventEmitter = approvalsports.EventEmitter

// AdmitFunc is an injection boundary for re-admission on modify.
type AdmitFunc = approvalsports.AdmitFunc

// ---------------------------------------------------------------------------
// Adapter type aliases and constants
// ---------------------------------------------------------------------------

// ExpiryWorker sweeps for expired pending approval_items and marks them expired.
type ExpiryWorker = approvalsadapters.ExpiryWorker

// DBVerifier adapts the *sql.DB-backed VerifyAndConsume to the Tier-0
// approvalsport.Verifier shape platform/toolgate depends on (D9).
type DBVerifier = approvalsadapters.DBVerifier

// Topic constants for approval lifecycle events.
const (
	TopicApprovalRequested = approvalsadapters.TopicApprovalRequested
	TopicApprovalDecided   = approvalsadapters.TopicApprovalDecided
)

// DefaultListPageLimit bounds a page when the caller passes a non-positive limit.
const DefaultListPageLimit = approvalsadapters.DefaultListPageLimit

// Binding is the Tier-0 approvalsport.Binding shape.
type Binding = approvalsport.Binding

// ---------------------------------------------------------------------------
// App type aliases and constants
// ---------------------------------------------------------------------------

// Decider executes approval decisions (approve / reject / modify).
type Decider = approvalsapp.Decider

// StageInput carries the parameters for a commit-block staging call.
type StageInput = approvalsapp.StageInput

// DefaultApprovalTTL is the expiry window applied to a staged approval_item
// when StageInput.TTL is zero.
const DefaultApprovalTTL = approvalsapp.DefaultApprovalTTL

// ---------------------------------------------------------------------------
// Domain function wrappers
// ---------------------------------------------------------------------------

// FlagEgress scans the send body and fieldMap keys against the §4.4 sensitivity
// map and returns a flag for each matching field.
func FlagEgress(body string, fieldMap map[string]string) []EgressFlag {
	return approvalsdomain.FlagEgress(body, fieldMap)
}

// ---------------------------------------------------------------------------
// Adapter function wrappers
// ---------------------------------------------------------------------------

// NewExpiryWorker returns an ExpiryWorker backed by db.
func NewExpiryWorker(db *sql.DB) *ExpiryWorker {
	return approvalsadapters.NewExpiryWorker(db)
}

// NewRepository returns a PostgreSQL-backed Repository.
//
//nolint:ireturn // seam returns the Repository interface by design
func NewRepository() Repository {
	return approvalsadapters.NewRepository()
}

// NewPageRepository returns the PostgreSQL-backed repository typed as the wider
// PageRepository (Repository + ListPage) for the /approvals read surface.
//
//nolint:ireturn // seam returns the PageRepository interface by design
func NewPageRepository() PageRepository {
	return approvalsadapters.NewPageRepository()
}

// SignToken mints a compact JWS from claims.
func SignToken(claims TokenClaims) (string, error) {
	return approvalsadapters.SignToken(claims)
}

// VerifyAndConsume validates a compact-JWS X-Approval-Token against the
// operation it must authorize, then atomically records its jti as consumed.
func VerifyAndConsume(ctx context.Context, db *sql.DB, token string, want Binding) error {
	return approvalsadapters.VerifyAndConsume(ctx, db, token, want)
}

// ---------------------------------------------------------------------------
// App function wrappers
// ---------------------------------------------------------------------------

// Stage is the commit-block: it writes a pending approval_item + one audit_log
// row (action=capture) in the caller's tx and returns ErrRequiresApproval.
func Stage(ctx context.Context, tx DBExec, repo Repository, in StageInput) (string, error) {
	return approvalsapp.Stage(ctx, tx, repo, in)
}

// ComputePreview returns a JSON snapshot of what an action would look like
// without performing any mutation.
func ComputePreview(ctx context.Context, p datasource.Provider, actionType string, payload json.RawMessage) (json.RawMessage, error) {
	return approvalsapp.ComputePreview(ctx, p, actionType, payload)
}
