// Package ports defines the approvals module's port interfaces.
package ports

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
)

// DBExec is satisfied by *sql.Tx and *sql.DB — the minimal exec/query surface
// needed by the approvals repository.
type DBExec interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

// Repository is the persistence seam for approval_item rows.
type Repository interface {
	Create(ctx context.Context, tx DBExec, it domain.Item) (string, error)
	Get(ctx context.Context, tx DBExec, id string) (domain.Item, error)
	ListByStatus(ctx context.Context, tx DBExec, workspaceID string, status domain.Status) ([]domain.Item, error)
	Transition(ctx context.Context, tx DBExec, id string, to domain.Status, decidedBy string) error
	// SetResumeWindow stores the runner's suspend-time window snapshot on a pending
	// item so a resumed run replays from exactly where it suspended. Idempotent.
	SetResumeWindow(ctx context.Context, tx DBExec, id string, window json.RawMessage) error
}

// PageRepository is Repository plus the cursor-paged inbox projection.
type PageRepository interface {
	Repository
	// ListPage is the cursor-paged sibling of ListByStatus for the /approvals inbox
	// projection: workspace-scoped, id-keyed cursor (afterID; "" starts at the head),
	// optional action_type (kind) filter, ORDER BY id. It returns the page (never nil)
	// plus the next-cursor id ("" when exhausted).
	ListPage(ctx context.Context, tx DBExec, workspaceID string, status domain.Status, kind, afterID string, limit int) ([]domain.Item, string, error)
}

// EventEmitter is a narrow seam for writing outbox events inside an open tx.
type EventEmitter interface {
	Emit(ctx context.Context, tx DBExec, topic, workspaceID, entityID string, payload json.RawMessage) error
}

// AdmitFunc is an injection boundary for re-admission on modify.
type AdmitFunc func(ctx context.Context, approverID string, actionType string, payload json.RawMessage) error
