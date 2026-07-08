package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

// ---------------------------------------------------------------------------
// ActivityStore — Update (partial update, task-field-vs-kind guard, audit +
// event write) and its private helpers, split out of store_activity.go to
// stay under the 500-LOC cap.
// ---------------------------------------------------------------------------

// Update applies partial updates to an activity.
// Handles: subject, body, remind_at, due_at, assignee_id, is_done/done_at.
// When ifMatch==0 the version check is skipped (last-write-wins).
// Returns the updated Activity.
func (s *ActivityStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Activity, error) {
	_, hasRemindAt := updates["remind_at"]
	_, hasDueAt := updates["due_at"]
	_, hasAssigneeID := updates["assignee_id"]
	_, hasIsDone := updates[fieldIsDone]

	remindAt := nullTime(updates, "remind_at")
	dueAt := nullTime(updates, "due_at")
	assigneeID := sqlutil.NullStr(updates, "assignee_id")

	// Resolve is_done value (JSON numbers come as float64).
	var isDoneVal *bool
	if hasIsDone {
		if v, ok := updates[fieldIsDone].(bool); ok {
			isDoneVal = &v
		}
	}

	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		if hasDueAt || hasAssigneeID || hasIsDone {
			if err := guardTaskFieldsAgainstKind(ctx, tx, id, workspaceID, dueAt, assigneeID, boolVal(isDoneVal)); err != nil {
				return err
			}
		}
		if err := s.applyUpdate(ctx, tx, id, workspaceID, updates, ifMatch,
			hasRemindAt, remindAt, hasDueAt, dueAt, hasAssigneeID, assigneeID, hasIsDone, boolVal(isDoneVal)); err != nil {
			return err
		}
		return s.writeUpdateAuditAndEvent(ctx, tx, id, workspaceID)
	})
	if err != nil {
		return domain.Activity{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// guardTaskFieldsAgainstKind enforces ACT-AC-11 on Update: when the caller is
// touching due_at/assignee_id/is_done, look up the row's kind and reject a
// task-only field on a non-task kind before the UPDATE runs. ErrNotFound if
// the row is absent (mirrors the UPDATE's own WHERE-clause not-found case).
func guardTaskFieldsAgainstKind(ctx context.Context, tx *sql.Tx, id, workspaceID string, dueAt *time.Time, assigneeID *string, isDone bool) error {
	var kind string
	if err := tx.QueryRowContext(ctx,
		`SELECT kind FROM activity WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
		id, workspaceID).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errs.ErrNotFound
		}
		return err
	}
	return validateTaskFields(kind, dueAt, assigneeID, isDone)
}

// applyUpdate runs the bounded UPDATE with the optimistic-concurrency guard
// folded into the WHERE clause: ifMatch==0 skips the version check
// (last-write-wins); a non-zero ifMatch requires the row version to match.
func (s *ActivityStore) applyUpdate(ctx context.Context, tx *sql.Tx, id, workspaceID string, updates map[string]any, ifMatch int64,
	hasRemindAt bool, remindAt *time.Time, hasDueAt bool, dueAt *time.Time, hasAssigneeID bool, assigneeID *string, hasIsDone bool, isDone bool,
) error {
	res, err := tx.ExecContext(ctx, `
			UPDATE activity
			SET subject     = COALESCE($3, subject),
			    body        = COALESCE($4, body),
			    remind_at   = CASE WHEN $5 THEN $6 ELSE remind_at END,
			    due_at      = CASE WHEN $7 THEN $8 ELSE due_at END,
			    assignee_id = CASE WHEN $9 THEN $10::uuid ELSE assignee_id END,
			    is_done     = CASE WHEN $11 THEN $12 ELSE is_done END,
			    done_at     = CASE
			                    WHEN $11 AND $12 THEN now()
			                    WHEN $11 AND NOT $12 THEN NULL
			                    ELSE done_at
			                  END,
			    updated_at  = now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
			  AND ($13 = 0 OR version = $13)`,
		id, workspaceID,
		sqlutil.NullStr(updates, "subject"),
		sqlutil.NullStr(updates, "body"),
		hasRemindAt, remindAt,
		hasDueAt, dueAt,
		hasAssigneeID, assigneeID,
		hasIsDone, isDone,
		ifMatch)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if ifMatch != 0 {
			return errs.ErrVersionSkew
		}
		return errs.ErrNotFound
	}
	return nil
}

// writeUpdateAuditAndEvent writes the one audit_log row (action=update) and
// one activity.updated event_outbox row for a successful Update call — one
// logical mutation, regardless of how many fields (including is_done) were
// touched in this request. ACT-EVT-N-1: the done-transition rides this same
// activity.updated event, never a distinct "task.completed" topic — there is
// no branch here for is_done, deliberately.
func (s *ActivityStore) writeUpdateAuditAndEvent(ctx context.Context, tx *sql.Tx, id, workspaceID string) error {
	payload, _ := json.Marshal(map[string]any{"activity_id": id})
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
		workspaceID, "activity.updated", id, payload); err != nil {
		return fmt.Errorf("activity update event: %w", err)
	}
	e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeActivity, &id, nil, nil)
	e.WorkspaceID = workspaceID
	if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
		return fmt.Errorf("activity update audit: %w", err)
	}
	return nil
}
