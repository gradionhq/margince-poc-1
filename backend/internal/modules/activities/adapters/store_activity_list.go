// Package adapters contains the SQL-backed implementations of the activities
// module's port interfaces.
package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

type activitySortSpec struct {
	column   string
	cursor   string
	orderDir string
}

func parseActivitySortSpec(sort string) activitySortSpec {
	spec := activitySortSpec{column: sortColOccurredAt, cursor: sortColOccurredAt, orderDir: "DESC"}
	if sort == "" {
		return spec
	}
	term := strings.TrimSpace(strings.Split(sort, ",")[0])
	if term == "" {
		return spec
	}
	if strings.HasPrefix(term, "-") {
		term = strings.TrimPrefix(term, "-")
	} else {
		spec.orderDir = "ASC"
	}
	switch term {
	case "occurred_at", "created_at":
		spec.column = term
		spec.cursor = term
	case "due_at":
		spec.column = dueAtSortExpr
		spec.cursor = dueAtSortExpr
	default:
		return activitySortSpec{column: sortColOccurredAt, cursor: sortColOccurredAt, orderDir: "DESC"}
	}
	return spec
}

func buildActivityListWhere(workspaceID, cursor string, f domain.ActivityListFilter) (string, []any, activitySortSpec, error) {
	spec := parseActivitySortSpec(f.Sort)
	args := []any{workspaceID}
	where := `a.workspace_id=$1::uuid`
	n := 1

	if !f.IncludeArchived {
		where += ` AND a.archived_at IS NULL`
	}
	if f.EntityType != "" && f.EntityID != "" {
		colName, ok := entityLinkColumn[f.EntityType]
		if !ok {
			return "", nil, spec, fmt.Errorf("unknown entity_type: %s", f.EntityType)
		}
		n++
		args = append(args, f.EntityID)
		where += fmt.Sprintf(` AND al.%s=$%d::uuid`, colName, n)
	}
	if f.Kind != "" {
		n++
		args = append(args, f.Kind)
		where += fmt.Sprintf(` AND a.kind=$%d`, n)
	}
	if f.AssigneeID != "" {
		n++
		args = append(args, f.AssigneeID)
		where += fmt.Sprintf(` AND a.assignee_id=$%d::uuid`, n)
	}
	if f.Direction != "" {
		n++
		args = append(args, f.Direction)
		where += fmt.Sprintf(` AND a.direction=$%d`, n)
	}
	if f.Q != "" {
		n++
		args = append(args, f.Q)
		where += fmt.Sprintf(` AND a.search_tsv @@ websearch_to_tsquery('simple', $%d)`, n)
	}

	curSortVal, curID, hasCursor := sqlutil.DecodeKeysetCursor(cursor)
	n++
	args = append(args, hasCursor)
	n++
	args = append(args, sqlutil.NullStrParam(curSortVal))
	n++
	args = append(args, sqlutil.NullStrParam(curID))
	where += fmt.Sprintf(` AND (NOT $%d OR (%s, a.id) %s ($%d::timestamptz, $%d::uuid))`, n-2, spec.cursor, map[bool]string{true: ">", false: "<"}[spec.orderDir == "ASC"], n-1, n)

	return where, args, spec, nil
}

// ListFiltered returns a cursor-keyed, workspace-scoped activity page with
// optional filters.
func (s *ActivityStore) ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, f domain.ActivityListFilter) ([]domain.Activity, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	where, args, spec, err := buildActivityListWhere(workspaceID, cursor, f)
	if err != nil {
		return nil, "", err
	}
	orderDir := spec.orderDir
	orderCol := spec.column

	out := []domain.Activity{}
	dbErr := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var query strings.Builder
		query.WriteString(`
			SELECT a.id, a.workspace_id, a.kind, a.subject, a.body,
			       a.occurred_at, a.due_at, a.assignee_id, a.remind_at, a.is_done, a.done_at,
			       a.duration_seconds, a.direction, a.meeting_status, a.source_system, a.source_id,
			       a.version, a.source, a.captured_by, a.created_at, a.updated_at
			FROM activity a`)
		if f.EntityType != "" && f.EntityID != "" {
			query.WriteString(` JOIN activity_link al ON al.activity_id = a.id`)
		}
		query.WriteString(` WHERE `)
		query.WriteString(where)
		_, _ = fmt.Fprintf(&query, ` ORDER BY %s %s, a.id %s LIMIT $%d`, orderCol, orderDir, orderDir, len(args)+1)
		args = append(args, limit+1)
		rows, err := tx.QueryContext(ctx, query.String(), args...)
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
			a.Links = []domain.ActivityLink{}
			out = append(out, a)
		}
		return rows.Err()
	})
	if dbErr != nil {
		return nil, "", dbErr
	}
	var next string
	if len(out) > limit {
		last := out[limit-1]
		sortVal := last.OccurredAt.UTC().Format(time.RFC3339Nano)
		switch spec.cursor {
		case "created_at":
			sortVal = last.CreatedAt.UTC().Format(time.RFC3339Nano)
		case dueAtSortExpr:
			if last.DueAt != nil {
				sortVal = last.DueAt.UTC().Format(time.RFC3339Nano)
			} else {
				sortVal = "infinity"
			}
		}
		next = sqlutil.EncodeKeysetCursor(sortVal, last.ID)
		out = out[:limit]
	}
	return out, next, nil
}

// List returns a keyset page of activities, optionally filtered to a linked entity, and the next cursor.
func (s *ActivityStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error) {
	return s.ListFiltered(ctx, workspaceID, cursor, limit, domain.ActivityListFilter{EntityType: entityType, EntityID: entityID})
}
