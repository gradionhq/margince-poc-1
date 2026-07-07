// Package adapters contains the SQL-backed implementations of the activities
// module's port interfaces.
package adapters

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/platform/workspacetx"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// entity-type constants used in the List activity_link JOIN.
const (
	entityTypePerson       = "person"
	entityTypeOrganization = "organization"
	entityTypeDeal         = "deal"
	fieldPersonID          = "person_id"
	fieldOrganizationID    = "organization_id"
	colDealID              = "deal_id"
	fieldIsDone            = "is_done"
)

// requireProvenance rejects an empty source or captured_by with a typed
// sentinel. HTTP handlers already reject empties at the edge, but non-HTTP
// callers (import/Datasource/direct store use) must not bypass the invariant.
func requireProvenance(source, capturedBy string) error {
	if source == "" || capturedBy == "" {
		return errs.ErrNullProvenance
	}
	return nil
}

// encodeKeysetCursor packs (sortVal, id) into one opaque, URL-safe token.
func encodeKeysetCursor(sortVal, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortVal + "\x00" + id))
}

// decodeKeysetCursor unpacks a token from encodeKeysetCursor. ok=false for an
// empty or malformed token — the caller treats it as "first page".
func decodeKeysetCursor(cursor string) (sortVal, id string, ok bool) {
	if cursor == "" {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// nullStrParam binds s as a SQL value, or NULL when s is empty.
func nullStrParam(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullStr(m map[string]any, key string) *string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

func nullTime(m map[string]any, key string) *time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case *time.Time:
			return t
		case time.Time:
			return &t
		case string:
			parsed, err := time.Parse(time.RFC3339, t)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func boolVal(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// ---------------------------------------------------------------------------
// ActivityStore
// ---------------------------------------------------------------------------

// ActivityStore manages activity rows.
type ActivityStore struct{ db *sql.DB }

// NewActivityStore returns an ActivityStore.
func NewActivityStore(db *sql.DB) *ActivityStore { return &ActivityStore{db: db} }

// Create inserts an activity in one workspace-scoped tx.
func (s *ActivityStore) Create(ctx context.Context, a domain.Activity) (domain.Activity, error) {
	if err := requireProvenance(a.Source, a.CapturedBy); err != nil {
		return domain.Activity{}, err
	}
	a.ID = ids.New()
	err := workspacetx.WithWorkspaceTx(ctx, s.db, a.WorkspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO activity (id, workspace_id, kind, subject, body,
			    occurred_at, due_at, assignee_id, remind_at, is_done, duration_seconds,
			    direction, meeting_status, source_system, source_id,
			    source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,1)`,
			a.ID, a.WorkspaceID, a.Kind, a.Subject, a.Body,
			a.OccurredAt, a.DueAt, a.AssigneeID, a.RemindAt, a.IsDone, a.DurationSeconds,
			a.Direction, a.MeetingStatus, a.SourceSystem, a.SourceID,
			a.Source, a.CapturedBy)
		return err
	})
	if err != nil {
		return domain.Activity{}, err
	}
	return s.Get(ctx, a.ID, a.WorkspaceID)
}

// Get returns one activity by id, workspace-scoped; ErrNotFound if absent.
func (s *ActivityStore) Get(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	var a domain.Activity
	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, kind, subject, body,
			       occurred_at, due_at, assignee_id, remind_at, is_done, done_at,
			       duration_seconds, direction, meeting_status, source_system, source_id,
			       transcript_ref, version, source, captured_by, created_at, updated_at, archived_at
			FROM activity WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&a.ID, &a.WorkspaceID, &a.Kind, &a.Subject, &a.Body,
			&a.OccurredAt, &a.DueAt, &a.AssigneeID, &a.RemindAt, &a.IsDone, &a.DoneAt,
			&a.DurationSeconds, &a.Direction, &a.MeetingStatus, &a.SourceSystem, &a.SourceID,
			&a.TranscriptRef, &a.Version, &a.Source, &a.CapturedBy,
			&a.CreatedAt, &a.UpdatedAt, &a.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return a, errs.ErrNotFound
	}
	return a, err
}

// List returns a keyset page of activities, optionally filtered to a linked entity, and the next cursor.
func (s *ActivityStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// ORDER BY occurred_at DESC, id — the seek must compare the FULL key, so a
	// page boundary mid-timestamp neither skips nor repeats rows. The opaque cursor
	// carries (occurred_at, id); the row-comparison predicate matches the ordering.
	curOccurred, curID, hasCursor := decodeKeysetCursor(cursor)

	out := []domain.Activity{}
	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var rows *sql.Rows
		var err error
		if entityType != "" && entityID != "" {
			// Timeline query via activity_link
			colName := map[string]string{
				entityTypePerson:       fieldPersonID,
				entityTypeOrganization: fieldOrganizationID,
				entityTypeDeal:         colDealID,
			}[entityType]
			if colName == "" {
				return fmt.Errorf("unknown entity_type: %s", entityType)
			}
			rows, err = tx.QueryContext(ctx, fmt.Sprintf(`
				SELECT a.id, a.workspace_id, a.kind, a.subject, a.body,
				       a.occurred_at, a.due_at, a.assignee_id, a.remind_at, a.is_done, a.done_at,
				       a.duration_seconds, a.direction, a.meeting_status, a.source_system, a.source_id,
				       a.version, a.source, a.captured_by, a.created_at, a.updated_at
				FROM activity a
				JOIN activity_link al ON al.activity_id = a.id
				WHERE a.workspace_id=$1::uuid AND al.%s=$2::uuid
				  AND a.archived_at IS NULL
				  AND (NOT $3 OR (a.occurred_at, a.id) < ($4::timestamptz, $5::uuid))
				ORDER BY a.occurred_at DESC, a.id DESC LIMIT $6`, colName),
				workspaceID, entityID, hasCursor, nullStrParam(curOccurred), nullStrParam(curID), limit+1)
		} else {
			rows, err = tx.QueryContext(ctx, `
				SELECT id, workspace_id, kind, subject, body,
				       occurred_at, due_at, assignee_id, remind_at, is_done, done_at,
				       duration_seconds, direction, meeting_status, source_system, source_id,
				       version, source, captured_by, created_at, updated_at
				FROM activity
				WHERE workspace_id=$1::uuid AND archived_at IS NULL
				  AND (NOT $2 OR (occurred_at, id) < ($3::timestamptz, $4::uuid))
				ORDER BY occurred_at DESC, id DESC LIMIT $5`,
				workspaceID, hasCursor, nullStrParam(curOccurred), nullStrParam(curID), limit+1)
		}
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var a domain.Activity
			if err := rows.Scan(&a.ID, &a.WorkspaceID, &a.Kind, &a.Subject, &a.Body,
				&a.OccurredAt, &a.DueAt, &a.AssigneeID, &a.RemindAt, &a.IsDone, &a.DoneAt,
				&a.DurationSeconds, &a.Direction, &a.MeetingStatus, &a.SourceSystem, &a.SourceID,
				&a.Version, &a.Source, &a.CapturedBy,
				&a.CreatedAt, &a.UpdatedAt); err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > limit {
		last := out[limit-1]
		next = encodeKeysetCursor(last.OccurredAt.UTC().Format(time.RFC3339Nano), last.ID)
		out = out[:limit]
	}
	return out, next, nil
}

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
	assigneeID := nullStr(updates, "assignee_id")

	// Resolve is_done value (JSON numbers come as float64).
	var isDoneVal *bool
	if hasIsDone {
		if v, ok := updates[fieldIsDone].(bool); ok {
			isDoneVal = &v
		}
	}

	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		// The optimistic-concurrency guard is folded into the WHERE: ifMatch==0 skips the
		// version check (last-write-wins); a non-zero ifMatch requires the row version to match.
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
			nullStr(updates, "subject"),
			nullStr(updates, "body"),
			hasRemindAt, remindAt,
			hasDueAt, dueAt,
			hasAssigneeID, assigneeID,
			hasIsDone, boolVal(isDoneVal),
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
	})
	if err != nil {
		return domain.Activity{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Archive soft-deletes an activity (sets archived_at).
func (s *ActivityStore) Archive(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE activity SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		return err
	})
	if err != nil {
		return domain.Activity{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches an activity by id regardless of archived_at status.
func (s *ActivityStore) getAny(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	var a domain.Activity
	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, kind, subject, body,
			       occurred_at, due_at, assignee_id, remind_at, is_done, done_at,
			       duration_seconds, direction, meeting_status, source_system, source_id,
			       version, source, captured_by, created_at, updated_at, archived_at
			FROM activity WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(
			&a.ID, &a.WorkspaceID, &a.Kind, &a.Subject, &a.Body,
			&a.OccurredAt, &a.DueAt, &a.AssigneeID, &a.RemindAt, &a.IsDone, &a.DoneAt,
			&a.DurationSeconds, &a.Direction, &a.MeetingStatus, &a.SourceSystem, &a.SourceID,
			&a.Version, &a.Source, &a.CapturedBy,
			&a.CreatedAt, &a.UpdatedAt, &a.ArchivedAt,
		)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return a, errs.ErrNotFound
	}
	return a, err
}
