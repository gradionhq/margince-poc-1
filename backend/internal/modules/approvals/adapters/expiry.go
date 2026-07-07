package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/riverqueue/river"

	"github.com/gradionhq/margince/backend/internal/modules/approvals/domain"
	"github.com/gradionhq/margince/backend/internal/modules/approvals/ports"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
)

// Internal audit-field constants for the expiry worker.
const (
	actorTypeSystem = "system"
	entityApproval  = "approval_item"
	decisionKey     = "decision"
	decisionExpired = "expired"
	itemIDKey       = "item_id"
)

// Topic constants for approval lifecycle events.
const (
	TopicApprovalRequested = "approval.requested"
	TopicApprovalDecided   = "approval.decided"
)

// ExpiryWorker sweeps for expired pending approval_items and marks them expired.
type ExpiryWorker struct {
	river.WorkerDefaults[domain.ExpiryArgs]
	db      *sql.DB
	Emitter ports.EventEmitter
}

// NewExpiryWorker returns an ExpiryWorker backed by db.
func NewExpiryWorker(db *sql.DB) *ExpiryWorker { return &ExpiryWorker{db: db} }

// Work runs the approval expiry sweep.
func (w *ExpiryWorker) Work(ctx context.Context, job *river.Job[domain.ExpiryArgs]) error {
	rows, err := w.db.QueryContext(ctx,
		`SELECT id::text, workspace_id::text FROM approval_item
		 WHERE status='pending' AND expires_at < now()`)
	if err != nil {
		return fmt.Errorf("expiry sweep query: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	type row struct{ id, wsID string }
	var expired []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.wsID); err != nil {
			return fmt.Errorf("expiry sweep scan: %w", err)
		}
		expired = append(expired, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("expiry sweep rows: %w", err)
	}
	_ = rows.Close()

	for _, r := range expired {
		if err := w.expireOne(ctx, r.id, r.wsID); err != nil {
			return err
		}
	}
	return nil
}

// expireOne transitions a single pending item to expired in its own tx.
func (w *ExpiryWorker) expireOne(ctx context.Context, id, workspaceID string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("expireOne begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.workspace_id', $1, true)`, workspaceID); err != nil {
		return fmt.Errorf("expireOne guc: %w", err)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE approval_item
		    SET status='expired', decided_by='system', decided_at=now()
		  WHERE id=$1::uuid AND status='pending'`, id)
	if err != nil {
		return fmt.Errorf("expireOne update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}

	authRule := "mcp.expiry"
	if _, err := crmaudit.WriteTx(ctx, tx, crmaudit.Entry{
		WorkspaceID:       workspaceID,
		ActorType:         actorTypeSystem,
		ActorID:           actorTypeSystem,
		Action:            decisionExpired,
		EntityType:        entityApproval,
		EntityID:          &id,
		After:             map[string]any{decisionKey: decisionExpired},
		AuthorizationRule: &authRule,
	}); err != nil {
		return fmt.Errorf("expireOne audit: %w", err)
	}

	if w.Emitter != nil {
		p, _ := json.Marshal(map[string]string{decisionKey: decisionExpired, itemIDKey: id})
		if err := w.Emitter.Emit(ctx, tx, TopicApprovalDecided, workspaceID, id, p); err != nil {
			return fmt.Errorf("expireOne emit: %w", err)
		}
	}

	return tx.Commit()
}
