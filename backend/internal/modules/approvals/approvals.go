// Package crmapprovals owns the approval-inbox for staged 🟡 MCP tool actions.
// It imports only crm-audit + datasource + errs + crmctx + ids (ADR-0014).
package crmapprovals

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Status is the lifecycle state of an approval_item row.
type Status string

// The approval_item lifecycle states.
const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusModified Status = "modified"
	StatusExpired  Status = "expired"
)

// Item is one approval_item row.
type Item struct {
	ID                 string
	WorkspaceID        string
	ActionType         string
	Payload            json.RawMessage
	DryRunPreview      json.RawMessage
	TrustTiers         json.RawMessage
	ContentEgressFlags json.RawMessage
	ResumeWindow       json.RawMessage
	Status             Status
	RequestedBy        string
	DecidedBy          *string
	DecidedAt          *time.Time
	ExpiresAt          *time.Time
	CreatedAt          time.Time
}

// DBExec is satisfied by *sql.Tx and *sql.DB — the minimal exec/query surface
// needed by the approvals repository.
type DBExec interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

// Repository is the persistence seam for approval_item rows.
type Repository interface {
	Create(ctx context.Context, tx DBExec, it Item) (string, error)
	Get(ctx context.Context, tx DBExec, id string) (Item, error)
	ListByStatus(ctx context.Context, tx DBExec, workspaceID string, status Status) ([]Item, error)
	Transition(ctx context.Context, tx DBExec, id string, to Status, decidedBy string) error
	// SetResumeWindow stores the runner's suspend-time window snapshot on a pending
	// item so a resumed run replays from exactly where it suspended. Idempotent.
	SetResumeWindow(ctx context.Context, tx DBExec, id string, window json.RawMessage) error
}

// PageRepository is Repository plus the cursor-paged inbox projection. The
// /approvals REST read surface depends on this wider seam; the staging/gate
// consumers keep using the narrower Repository, so their fakes need not grow a
// ListPage method (ListPage is additive — it never appears on Repository).
type PageRepository interface {
	Repository
	// ListPage is the cursor-paged sibling of ListByStatus for the /approvals inbox
	// projection: workspace-scoped, id-keyed cursor (afterID; "" starts at the head),
	// optional action_type (kind) filter, ORDER BY id. It returns the page (never nil)
	// plus the next-cursor id ("" when exhausted).
	ListPage(ctx context.Context, tx DBExec, workspaceID string, status Status, kind, afterID string, limit int) ([]Item, string, error)
}

// NewRepository returns a PostgreSQL-backed Repository.
//
//nolint:ireturn // seam returns the Repository interface by design
func NewRepository() Repository { return &pgRepository{} }

// NewPageRepository returns the PostgreSQL-backed repository typed as the wider
// PageRepository (Repository + ListPage) for the /approvals read surface.
//
//nolint:ireturn // seam returns the PageRepository interface by design
func NewPageRepository() PageRepository { return &pgRepository{} }

type pgRepository struct{}

func (r *pgRepository) Create(ctx context.Context, tx DBExec, it Item) (string, error) {
	expiresAt := it.ExpiresAt
	if expiresAt == nil {
		t := time.Now().Add(DefaultApprovalTTL)
		expiresAt = &t
	}
	var id string
	err := tx.QueryRowContext(
		ctx, `
		INSERT INTO approval_item
		  (workspace_id, action_type, payload, dry_run_preview, status, requested_by,
		   decided_by, decided_at, expires_at, trust_tiers, content_egress_flags, resume_window)
		VALUES ($1::uuid,$2,$3,$4,'pending',$5,$6,$7,$8::timestamptz,$9,$10,$11)
		RETURNING id`,
		it.WorkspaceID, it.ActionType, emptyObjectJSON(it.Payload), nullJSON(it.DryRunPreview),
		it.RequestedBy, it.DecidedBy, it.DecidedAt, expiresAt,
		nullJSON(it.TrustTiers), nullJSON(it.ContentEgressFlags), nullJSON(it.ResumeWindow),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("crmapprovals create: %w", err)
	}
	return id, nil
}

func (r *pgRepository) Get(ctx context.Context, tx DBExec, id string) (Item, error) {
	var it Item
	var decidedBy sql.NullString
	var decidedAt, expiresAt sql.NullTime
	var payload, preview, tiers, flags, window []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id::text, workspace_id::text, action_type, payload, dry_run_preview, status,
		       requested_by, decided_by, decided_at, expires_at, trust_tiers, content_egress_flags,
		       resume_window, created_at
		FROM approval_item WHERE id=$1::uuid`, id).Scan(
		&it.ID, &it.WorkspaceID, &it.ActionType, &payload, &preview, &it.Status,
		&it.RequestedBy, &decidedBy, &decidedAt, &expiresAt, &tiers, &flags, &window, &it.CreatedAt,
	)
	if err != nil {
		return Item{}, fmt.Errorf("crmapprovals get: %w", err)
	}
	it.Payload = json.RawMessage(payload)
	if len(preview) > 0 {
		it.DryRunPreview = json.RawMessage(preview)
	}
	if len(tiers) > 0 {
		it.TrustTiers = json.RawMessage(tiers)
	}
	if len(flags) > 0 {
		it.ContentEgressFlags = json.RawMessage(flags)
	}
	if len(window) > 0 {
		it.ResumeWindow = json.RawMessage(window)
	}
	if decidedBy.Valid {
		it.DecidedBy = &decidedBy.String
	}
	if decidedAt.Valid {
		it.DecidedAt = &decidedAt.Time
	}
	if expiresAt.Valid {
		it.ExpiresAt = &expiresAt.Time
	}
	return it, nil
}

func (r *pgRepository) ListByStatus(ctx context.Context, tx DBExec, workspaceID string, status Status) ([]Item, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text, workspace_id::text, action_type, payload, status, requested_by,
		       decided_by, decided_at, expires_at
		FROM approval_item
		WHERE workspace_id=$1::uuid AND status=$2
		ORDER BY created_at`, workspaceID, string(status))
	if err != nil {
		return nil, fmt.Errorf("crmapprovals list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var items []Item
	for rows.Next() {
		var it Item
		var decidedBy sql.NullString
		var decidedAt, expiresAt sql.NullTime
		var payload []byte
		if err := rows.Scan(&it.ID, &it.WorkspaceID, &it.ActionType, &payload, &it.Status,
			&it.RequestedBy, &decidedBy, &decidedAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("crmapprovals list scan: %w", err)
		}
		it.Payload = json.RawMessage(payload)
		if decidedBy.Valid {
			it.DecidedBy = &decidedBy.String
		}
		if decidedAt.Valid {
			it.DecidedAt = &decidedAt.Time
		}
		if expiresAt.Valid {
			it.ExpiresAt = &expiresAt.Time
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// DefaultListPageLimit bounds a page when the caller passes a non-positive limit.
const DefaultListPageLimit = 50

func (r *pgRepository) ListPage(ctx context.Context, tx DBExec, workspaceID string, status Status, kind, afterID string, limit int) ([]Item, string, error) {
	if limit <= 0 {
		limit = DefaultListPageLimit
	}
	// Same projection columns as ListByStatus; id-keyed cursor + optional kind filter.
	q := `
		SELECT id::text, workspace_id::text, action_type, payload, status, requested_by,
		       decided_by, decided_at, expires_at, created_at
		FROM approval_item
		WHERE workspace_id=$1::uuid AND status=$2`
	args := []any{workspaceID, string(status)}
	if afterID != "" {
		args = append(args, afterID)
		q += fmt.Sprintf(" AND id > $%d::uuid", len(args))
	}
	if kind != "" {
		args = append(args, kind)
		q += fmt.Sprintf(" AND action_type = $%d", len(args))
	}
	// Fetch limit+1 so we can detect (and emit a cursor for) a next page.
	args = append(args, limit+1)
	q += fmt.Sprintf(" ORDER BY id LIMIT $%d", len(args))

	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("crmapprovals list page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []Item{}
	for rows.Next() {
		var it Item
		var decidedBy sql.NullString
		var decidedAt, expiresAt sql.NullTime
		var payload []byte
		if err := rows.Scan(&it.ID, &it.WorkspaceID, &it.ActionType, &payload, &it.Status,
			&it.RequestedBy, &decidedBy, &decidedAt, &expiresAt, &it.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("crmapprovals list page scan: %w", err)
		}
		it.Payload = json.RawMessage(payload)
		if decidedBy.Valid {
			it.DecidedBy = &decidedBy.String
		}
		if decidedAt.Valid {
			it.DecidedAt = &decidedAt.Time
		}
		if expiresAt.Valid {
			it.ExpiresAt = &expiresAt.Time
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("crmapprovals list page rows: %w", err)
	}
	// More than `limit` rows means a next page exists; the cursor is the page's last id.
	next := ""
	if len(items) > limit {
		next = items[limit-1].ID
		items = items[:limit]
	}
	return items, next, nil
}

func (r *pgRepository) Transition(ctx context.Context, tx DBExec, id string, to Status, decidedBy string) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE approval_item
		   SET status=$1, decided_by=$2, decided_at=now()
		 WHERE id=$3::uuid AND status='pending'`, string(to), decidedBy, id)
	if err != nil {
		return fmt.Errorf("crmapprovals transition: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("crmapprovals transition: no pending row for id=%s", id)
	}
	return nil
}

func (r *pgRepository) SetResumeWindow(ctx context.Context, tx DBExec, id string, window json.RawMessage) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE approval_item SET resume_window=$2 WHERE id=$1::uuid`, id, nullJSON(window)); err != nil {
		return fmt.Errorf("crmapprovals set resume_window: %w", err)
	}
	return nil
}

func nullJSON(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

// emptyObjectJSON coalesces an absent payload to an empty JSON object for the
// NOT-NULL payload column: a yellow tool can be staged with no args (e.g. a
// parameterless action), and a staged item must still carry a payload.
func emptyObjectJSON(b json.RawMessage) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return []byte(b)
}
