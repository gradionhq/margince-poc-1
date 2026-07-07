// Package adapters — minimal PersonStore for org-strength aggregation.
// Only the strengthActivitiesFor method is needed by OrgStore; the full
// PersonStore with CRUD lives in modules/directory. This is a minimal
// collaborator, not a full person store.
package adapters

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

// PersonStore is a minimal store used by OrgStore to fetch person-level
// strength-activity data for org-strength aggregation (PO-N-ORGSTRENGTH).
// It is not a general person-CRUD store; that lives in modules/directory.
type PersonStore struct{ db *sql.DB }

// NewPersonStore returns a PersonStore backed by db.
func NewPersonStore(db *sql.DB) *PersonStore { return &PersonStore{db: db} }

// strengthActivitiesFor batch-fetches every live email/call/meeting activity
// linked to any of personIDs, grouped by person_id. Mirrors
// directory.PersonStore.strengthActivitiesFor exactly (same query, same shape).
func (s *PersonStore) strengthActivitiesFor(ctx context.Context, tx *sql.Tx, workspaceID string, personIDs []string) (map[string][]strength.Activity, error) {
	out := map[string][]strength.Activity{}
	if len(personIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT al.person_id, a.id, a.kind, a.subject, a.occurred_at, a.direction
		FROM activity a
		JOIN activity_link al ON al.activity_id = a.id
		WHERE a.workspace_id=$1::uuid AND a.archived_at IS NULL
		  AND al.person_id = ANY($2::uuid[])
		  AND a.kind IN ('email','call','meeting')`,
		workspaceID, pq.Array(personIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var personID string
		var a strength.Activity
		if err := rows.Scan(&personID, &a.ID, &a.Kind, &a.Subject, &a.OccurredAt, &a.Direction); err != nil {
			return nil, err
		}
		out[personID] = append(out[personID], a)
	}
	return out, rows.Err()
}
