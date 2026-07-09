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

// activitySortSpec is the store-layer-parsed result of a (possibly empty)
// `sort` filter value. sortCol is bound as a query parameter and compared
// inside a SQL CASE expression (see activityListQueryAsc/Desc below) — it is
// NEVER interpolated into query text, so no dynamically-formatted SQL string
// is ever built at runtime (avoids SonarCloud go:S2077; mirrors the
// records/adapters/quota.go List precedent of a single static query with
// bound-value guards).
// sortColCreatedAt/sortColDueAt are the two non-default activitySortSpec.sortCol
// values (occurred_at is the zero value "") — named consts so the same literal
// isn't repeated across the sort-term switch and the cursor-formatting switch
// (goconst).
const (
	sortColCreatedAt = "created_at"
	sortColDueAt     = "due_at"
)

type activitySortSpec struct {
	sortCol string // "" (occurred_at, the default), sortColCreatedAt, or sortColDueAt
	desc    bool
}

// parseActivitySortSpec parses the first comma-separated sort term (id is
// always the implicit final tie-breaker). An unrecognized term falls back to
// the default -occurred_at — this is defense in depth; the user-facing 422
// sort_field_not_allowed lives in the handler (activitySortColumnsMap).
func parseActivitySortSpec(sort string) activitySortSpec {
	spec := activitySortSpec{desc: true}
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
		spec.desc = false
	}
	switch term {
	case sortColCreatedAt:
		spec.sortCol = sortColCreatedAt
	case sortColDueAt:
		spec.sortCol = sortColDueAt
	case "occurred_at":
		spec.sortCol = ""
	default:
		return activitySortSpec{desc: true}
	}
	return spec
}

// buildActivityListArgs validates f.EntityType (a plain Go conditional — no
// SQL text is built from it) and assembles the bound-parameter list for
// activityListQueryAsc/Desc, in the exact $1.. order those queries expect.
func buildActivityListArgs(workspaceID, cursor string, limit int, f domain.ActivityListFilter) ([]any, activitySortSpec, error) {
	if f.EntityType != "" {
		if _, ok := entityLinkColumn[f.EntityType]; !ok {
			return nil, activitySortSpec{}, fmt.Errorf("unknown entity_type: %s", f.EntityType)
		}
	}
	spec := parseActivitySortSpec(f.Sort)
	curSortVal, curID, hasCursor := sqlutil.DecodeKeysetCursor(cursor)
	args := []any{
		workspaceID,                      // $1
		f.IncludeArchived,                // $2
		f.Kind,                           // $3
		f.AssigneeID,                     // $4
		f.Direction,                      // $5
		f.Q,                              // $6
		f.EntityType,                     // $7
		f.EntityID,                       // $8
		hasCursor,                        // $9
		spec.sortCol,                     // $10
		sqlutil.NullStrParam(curSortVal), // $11
		sqlutil.NullStrParam(curID),      // $12
		limit + 1,                        // $13
	}
	return args, spec, nil
}

// activityListColumns is the column list shared by both query variants below
// — kept as one named literal so the two variants can never drift apart.
const activityListColumns = `
	SELECT a.id, a.workspace_id, a.kind, a.subject, a.body,
	       a.occurred_at, a.due_at, a.assignee_id, a.remind_at, a.is_done, a.done_at,
	       a.duration_seconds, a.direction, a.meeting_status, a.source_system, a.source_id,
	       a.version, a.source, a.captured_by, a.created_at, a.updated_at, a.archived_at
	FROM activity a`

// activityListWhere is the fully static filter clause shared by both query
// variants. Every optional filter is applied via an always-bound
// empty-string/boolean guard on its parameter (mirrors
// records/adapters/quota.go's List — no WHERE text is built at runtime).
// The entity_type/entity_id pair is resolved via three literal EXISTS
// branches (one per activity_link FK column) gated by comparing the bound
// $7 value against the fixed literals 'person'/'organization'/'deal' —
// again, no column name is ever interpolated into the query text. Postgres
// short-circuits AND/OR at execution, so a $8::uuid cast is only reached
// once $7 has already matched that branch's literal (never on an empty or
// mismatched entity_type/entity_id pair) — the same "both non-empty to
// filter" permissive behavior the legacy List always had.
// The sort/keyset column choice is resolved via a CASE on the bound $10
// value (never string-formatted into the query), with due_at's COALESCE to
// 'infinity' keeping the ordering total without excluding NULL rows.
const activityListWhere = `
	WHERE a.workspace_id=$1::uuid
	  AND ($2 OR a.archived_at IS NULL)
	  AND ($3 = '' OR a.kind=$3)
	  AND ($4 = '' OR a.assignee_id=$4::uuid)
	  AND ($5 = '' OR a.direction=$5)
	  AND ($6 = '' OR a.search_tsv @@ websearch_to_tsquery('simple', $6))
	  AND ($7 = '' OR $8 = '' OR
	        ($7='person' AND EXISTS (SELECT 1 FROM activity_link al WHERE al.activity_id=a.id AND al.person_id=$8::uuid)) OR
	        ($7='organization' AND EXISTS (SELECT 1 FROM activity_link al WHERE al.activity_id=a.id AND al.organization_id=$8::uuid)) OR
	        ($7='deal' AND EXISTS (SELECT 1 FROM activity_link al WHERE al.activity_id=a.id AND al.deal_id=$8::uuid)))`

// activityListQueryDesc / activityListQueryAsc are the only two query
// variants ListFiltered ever executes — chosen via a plain Go if/else
// assignment between these two pre-declared literals (mirrors
// offers/adapters/store_offer_template.go's offerTemplateListQueryLive/All),
// never built via fmt.Sprintf/strings.Builder, so no dynamically-formatted
// SQL string reaches tx.QueryContext (SonarCloud go:S2077).
const activityListQueryDesc = activityListColumns + activityListWhere + `
	  AND (NOT $9 OR
	        (CASE WHEN $10='created_at' THEN a.created_at
	              WHEN $10='due_at' THEN COALESCE(a.due_at, 'infinity'::timestamptz)
	              ELSE a.occurred_at END, a.id) < ($11::timestamptz, $12::uuid))
	ORDER BY (CASE WHEN $10='created_at' THEN a.created_at
	               WHEN $10='due_at' THEN COALESCE(a.due_at, 'infinity'::timestamptz)
	               ELSE a.occurred_at END) DESC, a.id DESC
	LIMIT $13`

const activityListQueryAsc = activityListColumns + activityListWhere + `
	  AND (NOT $9 OR
	        (CASE WHEN $10='created_at' THEN a.created_at
	              WHEN $10='due_at' THEN COALESCE(a.due_at, 'infinity'::timestamptz)
	              ELSE a.occurred_at END, a.id) > ($11::timestamptz, $12::uuid))
	ORDER BY (CASE WHEN $10='created_at' THEN a.created_at
	               WHEN $10='due_at' THEN COALESCE(a.due_at, 'infinity'::timestamptz)
	               ELSE a.occurred_at END) ASC, a.id ASC
	LIMIT $13`

// ListFiltered returns a cursor-keyed, workspace-scoped activity page with
// optional filters (ACT-WIRE-1, DM-VOCAB-4).
func (s *ActivityStore) ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, f domain.ActivityListFilter) ([]domain.Activity, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	args, spec, err := buildActivityListArgs(workspaceID, cursor, limit, f)
	if err != nil {
		return nil, "", err
	}
	query := activityListQueryDesc
	if !spec.desc {
		query = activityListQueryAsc
	}

	out := []domain.Activity{}
	dbErr := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, query, args...)
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
				&a.CreatedAt, &a.UpdatedAt, &a.ArchivedAt); err != nil {
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
		switch spec.sortCol {
		case sortColCreatedAt:
			sortVal = last.CreatedAt.UTC().Format(time.RFC3339Nano)
		case sortColDueAt:
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
// Delegates to ListFiltered with a filter carrying only the entity predicate, preserving this exact
// signature — people/organizations/deals transport handlers call it directly for their 360-composite reads.
func (s *ActivityStore) List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int) ([]domain.Activity, string, error) {
	return s.ListFiltered(ctx, workspaceID, cursor, limit, domain.ActivityListFilter{EntityType: entityType, EntityID: entityID})
}
