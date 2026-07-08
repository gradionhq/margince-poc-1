// Package adapters contains the SQL-backed implementations of the activities
// module's port interfaces.
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
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

// entityLinkColumn maps an activity_link entity_type to its FK column —
// shared by List's timeline JOIN and Create's link-insert.
var entityLinkColumn = map[string]string{
	entityTypePerson:       fieldPersonID,
	entityTypeOrganization: fieldOrganizationID,
	entityTypeDeal:         colDealID,
}

// Generic, domain-free store helpers (provenance guard, keyset/offset cursors,
// bounded-update field readers) live in the Tier-0 shared/kernel/sqlutil package.

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

// validateTaskFields enforces the activity_task_fields DB CHECK (kind='task' OR
// (due_at IS NULL AND assignee_id IS NULL AND is_done=false)) at the application
// boundary, so a request that would trip that CHECK fails 422
// field_not_valid_for_kind instead of a 500 from an unmapped constraint
// violation (ACT-AC-11). Scoped to exactly the three columns the CHECK covers —
// remind_at/done_at/meeting_status/duration_seconds/direction are documented as
// kind-scoped in the Activity schema's own comment but are not DB-enforced, so
// validating them is a separate, unscoped concern.
func validateTaskFields(kind string, dueAt *time.Time, assigneeID *string, isDone bool) error {
	if kind == "task" {
		return nil
	}
	if dueAt != nil || assigneeID != nil || isDone {
		return errs.ErrFieldNotValidForKind
	}
	return nil
}

// jsonbParam binds v as a jsonb query parameter, or SQL NULL when v is nil —
// distinct from sqlutil.MarshalJSON's always-{} behavior, since activity.raw is
// nullable and null is a meaningfully different state from an empty object.
func jsonbParam(v map[string]any) any {
	if v == nil {
		return nil
	}
	return sqlutil.MarshalJSON(v)
}

// insertLinks writes one activity_link row per link, in the caller's transaction.
// Only called for a freshly-inserted activity — an idempotent replay leaves the
// original row's links untouched (ACT-WIRE-2).
func (s *ActivityStore) insertLinks(ctx context.Context, tx *sql.Tx, activityID, workspaceID string, links []domain.ActivityLink) error {
	for _, l := range links {
		// entityLinkColumn is still consulted for its "unknown entity_type"
		// error path, but its string result is never interpolated into SQL —
		// insertLink below dispatches on l.EntityType against a fixed set of
		// literal INSERT statements instead, so no query text is built from
		// runtime data (avoids a SonarCloud dynamic-SQL hotspot).
		if _, ok := entityLinkColumn[l.EntityType]; !ok {
			return fmt.Errorf("unknown activity_link entity_type: %s", l.EntityType)
		}
		if err := s.insertLink(ctx, tx, activityID, workspaceID, l); err != nil {
			return err
		}
	}
	return nil
}

// insertLink writes a single activity_link row using one of three literal
// (non-interpolated) INSERT statements, one per FK column — see insertLinks.
func (s *ActivityStore) insertLink(ctx context.Context, tx *sql.Tx, activityID, workspaceID string, l domain.ActivityLink) error {
	var err error
	switch l.EntityType {
	case entityTypePerson:
		_, err = tx.ExecContext(ctx,
			`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, person_id) VALUES ($1,$2,$3,$4,$5)`,
			ids.New(), workspaceID, activityID, l.EntityType, l.EntityID)
	case entityTypeOrganization:
		_, err = tx.ExecContext(ctx,
			`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, organization_id) VALUES ($1,$2,$3,$4,$5)`,
			ids.New(), workspaceID, activityID, l.EntityType, l.EntityID)
	case entityTypeDeal:
		_, err = tx.ExecContext(ctx,
			`INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, deal_id) VALUES ($1,$2,$3,$4,$5)`,
			ids.New(), workspaceID, activityID, l.EntityType, l.EntityID)
	default:
		// Unreachable: insertLinks already validated l.EntityType against
		// entityLinkColumn before calling insertLink.
		return fmt.Errorf("unknown activity_link entity_type: %s", l.EntityType)
	}
	return err
}

// Create inserts an activity in one workspace-scoped tx. When a.SourceSystem
// and a.SourceID are both present, the insert is idempotent against
// uq_activity_source: a replay of the same (source_system, source_id) pair
// resolves to the existing row (created=false) instead of a duplicate or a
// unique-violation error (ACT-WIRE-2). A fresh insert also writes one
// activity_link row per a.Links entry (ACT-AC-1) and a.Raw into the raw jsonb
// column (ACT-AC-2). Rejects missing provenance (ACT-AC-3) and a task-only
// field set on a non-task kind (ACT-AC-11) before ever reaching the DB.
func (s *ActivityStore) Create(ctx context.Context, a domain.Activity) (domain.Activity, bool, error) {
	if err := sqlutil.RequireProvenance(a.Source, a.CapturedBy); err != nil {
		return domain.Activity{}, false, err
	}
	if err := validateTaskFields(a.Kind, a.DueAt, a.AssigneeID, a.IsDone); err != nil {
		return domain.Activity{}, false, err
	}
	a.ID = ids.New()
	var created bool
	err := database.WithWorkspaceTx(ctx, s.db, a.WorkspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO activity (id, workspace_id, kind, subject, body,
			    occurred_at, due_at, assignee_id, remind_at, is_done, duration_seconds,
			    direction, meeting_status, source_system, source_id,
			    source, captured_by, raw, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,1)
			ON CONFLICT (workspace_id, source_system, source_id)
			    WHERE source_system IS NOT NULL AND source_id IS NOT NULL AND archived_at IS NULL DO NOTHING`,
			a.ID, a.WorkspaceID, a.Kind, a.Subject, a.Body,
			a.OccurredAt, a.DueAt, a.AssigneeID, a.RemindAt, a.IsDone, a.DurationSeconds,
			a.Direction, a.MeetingStatus, a.SourceSystem, a.SourceID,
			a.Source, a.CapturedBy, jsonbParam(a.Raw))
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		created = n > 0
		if !created {
			// Idempotent replay: the row already exists under this source key.
			return tx.QueryRowContext(ctx,
				`SELECT id FROM activity WHERE workspace_id=$1 AND source_system=$2 AND source_id=$3`,
				a.WorkspaceID, a.SourceSystem, a.SourceID).Scan(&a.ID)
		}
		return s.insertLinks(ctx, tx, a.ID, a.WorkspaceID, a.Links)
	})
	if err != nil {
		return domain.Activity{}, false, err
	}
	got, err := s.Get(ctx, a.ID, a.WorkspaceID)
	return got, created, err
}

// Get returns one activity by id, workspace-scoped, including its typed links
// and raw capture payload; ErrNotFound if absent (ACT-WIRE-3).
func (s *ActivityStore) Get(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	var a domain.Activity
	var rawBytes []byte
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		scanErr := tx.QueryRowContext(ctx, `
			SELECT id, workspace_id, kind, subject, body,
			       occurred_at, due_at, assignee_id, remind_at, is_done, done_at,
			       duration_seconds, direction, meeting_status, source_system, source_id,
			       transcript_ref, raw, version, source, captured_by, created_at, updated_at, archived_at
			FROM activity WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID).Scan(
			&a.ID, &a.WorkspaceID, &a.Kind, &a.Subject, &a.Body,
			&a.OccurredAt, &a.DueAt, &a.AssigneeID, &a.RemindAt, &a.IsDone, &a.DoneAt,
			&a.DurationSeconds, &a.Direction, &a.MeetingStatus, &a.SourceSystem, &a.SourceID,
			&a.TranscriptRef, &rawBytes, &a.Version, &a.Source, &a.CapturedBy,
			&a.CreatedAt, &a.UpdatedAt, &a.ArchivedAt,
		)
		if scanErr != nil {
			return scanErr
		}
		links, linkErr := s.selectLinks(ctx, tx, a.ID)
		if linkErr != nil {
			return linkErr
		}
		a.Links = links
		return nil
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Activity{}, errs.ErrNotFound
	}
	if err != nil {
		return domain.Activity{}, err
	}
	if rawBytes != nil {
		sqlutil.UnmarshalJSON(rawBytes, &a.Raw)
	}
	return a, nil
}

// selectLinks reads every activity_link row for id, in the caller's transaction.
func (s *ActivityStore) selectLinks(ctx context.Context, tx *sql.Tx, activityID string) ([]domain.ActivityLink, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, entity_type, coalesce(person_id, organization_id, deal_id) FROM activity_link WHERE activity_id=$1`,
		activityID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	links := []domain.ActivityLink{}
	for rows.Next() {
		var l domain.ActivityLink
		l.ActivityID = activityID
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// List returns a keyset page of activities, optionally filtered to a linked entity, and the next cursor.
func (s *ActivityStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// ORDER BY occurred_at DESC, id — the seek must compare the FULL key, so a
	// page boundary mid-timestamp neither skips nor repeats rows. The opaque cursor
	// carries (occurred_at, id); the row-comparison predicate matches the ordering.
	curOccurred, curID, hasCursor := sqlutil.DecodeKeysetCursor(cursor)

	out := []domain.Activity{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var rows *sql.Rows
		var err error
		if entityType != "" && entityID != "" {
			// Timeline query via activity_link
			colName := entityLinkColumn[entityType]
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
				workspaceID, entityID, hasCursor, sqlutil.NullStrParam(curOccurred), sqlutil.NullStrParam(curID), limit+1)
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
				workspaceID, hasCursor, sqlutil.NullStrParam(curOccurred), sqlutil.NullStrParam(curID), limit+1)
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
			// List never selects links (or raw, by design — see
			// TestActivityStore_List_NeverSelectsRaw), but Activity.Links has no
			// omitempty and must marshal to JSON "[]", not "null" — so it needs an
			// explicit non-nil zero value here, mirroring selectLinks' initializer.
			a.Links = []domain.ActivityLink{}
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
		next = sqlutil.EncodeKeysetCursor(last.OccurredAt.UTC().Format(time.RFC3339Nano), last.ID)
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
		return s.applyUpdate(ctx, tx, id, workspaceID, updates, ifMatch,
			hasRemindAt, remindAt, hasDueAt, dueAt, hasAssigneeID, assigneeID, hasIsDone, boolVal(isDoneVal))
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

// Archive soft-deletes an activity (sets archived_at).
func (s *ActivityStore) Archive(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE activity SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE attachment SET archived_at=now()
				 WHERE entity_type='activity' AND entity_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
				id, workspaceID); err != nil {
				return fmt.Errorf("activity archive attachment cascade: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Activity{}, err
	}
	return s.getAny(ctx, id, workspaceID)
}

// getAny fetches an activity by id regardless of archived_at status.
func (s *ActivityStore) getAny(ctx context.Context, id, workspaceID string) (domain.Activity, error) {
	var a domain.Activity
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
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

// ProvenanceRatio reports the count of live (non-archived) activities in
// workspaceID captured by an agent vs. a human — the manual-entry-smell ratio
// (ACT-AC-7). Keys on the "agent:"/"human:" captured_by prefix convention
// (crm.yaml logActivity examples).
func (s *ActivityStore) ProvenanceRatio(ctx context.Context, workspaceID string) (agentCount, humanCount int, err error) {
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT
			    count(*) FILTER (WHERE captured_by LIKE 'agent:%'),
			    count(*) FILTER (WHERE captured_by LIKE 'human:%')
			FROM activity WHERE workspace_id=$1::uuid AND archived_at IS NULL`,
			workspaceID).Scan(&agentCount, &humanCount)
	})
	return agentCount, humanCount, err
}
