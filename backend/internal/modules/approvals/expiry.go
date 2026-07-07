package crmapprovals

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/riverqueue/river"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// ExpiryArgs is the River job payload for the approval expiry sweep.
type ExpiryArgs struct{}

// Kind implements river.JobArgs.
func (ExpiryArgs) Kind() string { return "approval_expiry_sweep" }

// ExpiryWorker sweeps for expired pending approval_items and marks them expired.
type ExpiryWorker struct {
	river.WorkerDefaults[ExpiryArgs]
	db      *sql.DB
	Emitter EventEmitter
}

// NewExpiryWorker returns an ExpiryWorker backed by db.
func NewExpiryWorker(db *sql.DB) *ExpiryWorker { return &ExpiryWorker{db: db} }

// Work runs the approval expiry sweep. It reads all pending items past their
// expires_at without RLS (privileged sweep), then transitions each one.
//
// The unscoped read is INTENTIONAL: this is a single global background worker that
// must fail-close expired items across every workspace (a per-workspace fan-out
// would need a workspace enumerator the sweep doesn't own). It fails closed
// (pending→expired, never auto-commit) and writes a per-item audit row under each
// item's own workspace GUC below, so the cross-workspace read is a worker-scope
// read, not a tenant-isolation hole.
func (w *ExpiryWorker) Work(ctx context.Context, job *river.Job[ExpiryArgs]) error {
	// rls-exempt: intentional cross-workspace privileged sweep — single global worker sees every workspace's expired items; per-item writes go through expireOne's own WithWorkspaceTx.
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
	return database.WithWorkspaceTx(ctx, w.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE approval_item
			    SET status='expired', decided_by='system', decided_at=now()
			  WHERE id=$1::uuid AND status='pending'`, id)
		if err != nil {
			return fmt.Errorf("expireOne update: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			// Already transitioned by another process; skip quietly.
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
		return nil
	})
}
