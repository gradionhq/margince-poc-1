package adapters

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/people/domain"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

// ---------------------------------------------------------------------------
// PersonStore — PO-F-3 relationship strength
// ---------------------------------------------------------------------------

// scanAllLivePersons runs personListColumns' fixed id-ordered SELECT and
// scans every live workspace row (including its active custom-field values)
// into a slice — the row-fetch step listByStrength shares with listByID's
// non-strength cases, factored out so listByStrength itself stays under the
// file-length/funlen cap.
func scanAllLivePersons(ctx context.Context, tx *sql.Tx, workspaceID string, active []customfields.Column) ([]domain.Person, error) {
	// Non-nil so an empty result marshals to a JSON array ([]), never null.
	all := []domain.Person{}
	//nolint:gosec // G202: personListColumns/customfields.SelectSuffix return quoted, catalog-derived identifiers only, never user input
	query := personListColumns + customfields.SelectSuffix(active) + `, version, source, captured_by, created_at, updated_at
		FROM person
		WHERE workspace_id=$1::uuid AND archived_at IS NULL
		ORDER BY id`
	rows, err := tx.QueryContext(ctx, query, workspaceID) // NOSONAR: query is built from a fixed literal + customfields.SelectSuffix's quoted, catalog-derived identifiers only; workspaceID is a bound param
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		p, err := scanPersonListRow(rows, active)
		if err != nil {
			return nil, err
		}
		all = append(all, p)
	}
	return all, rows.Err()
}

func (s *PersonStore) listByStrength(ctx context.Context, workspaceID, cursor string, limit int, ascending bool) ([]domain.Person, string, error) {
	offset := sqlutil.DecodeOffsetCursor(cursor)
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "person")
	if err != nil {
		return nil, "", err
	}
	var all []domain.Person
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var scanErr error
		all, scanErr = scanAllLivePersons(ctx, tx, workspaceID, active)
		if scanErr != nil {
			return scanErr
		}
		ptrs := make([]*domain.Person, len(all))
		for i := range all {
			ptrs[i] = &all[i]
		}
		if err := s.attachStrength(ctx, tx, workspaceID, ptrs); err != nil {
			return err
		}
		return s.attachLastActivity(ctx, tx, workspaceID, ptrs)
	})
	if err != nil {
		return nil, "", err
	}

	sort.SliceStable(all, func(i, j int) bool {
		si, sj := all[i].Strength, all[j].Strength
		if si == nil && sj == nil {
			return all[i].ID < all[j].ID
		}
		if si == nil {
			return false
		}
		if sj == nil {
			return true
		}
		if si.Score != sj.Score {
			if ascending {
				return si.Score < sj.Score
			}
			return si.Score > sj.Score
		}
		return all[i].ID < all[j].ID
	})

	if offset > len(all) {
		offset = len(all)
	}
	end := offset + limit
	var next string
	if end < len(all) {
		next = sqlutil.EncodeOffsetCursor(end)
	} else {
		end = len(all)
	}
	return all[offset:end], next, nil
}

// strengthActivitiesFor batch-fetches every live email/call/meeting activity
// linked to any of personIDs, grouped by person_id.
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

// lastActivityFor batch-fetches the most recent live activity timestamp per
// person, across every activity kind. Honest omission when no activity recorded.
func (s *PersonStore) lastActivityFor(ctx context.Context, tx *sql.Tx, workspaceID string, personIDs []string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	if len(personIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT al.person_id, MAX(a.occurred_at)
		FROM activity a
		JOIN activity_link al ON al.activity_id = a.id
		WHERE a.workspace_id=$1::uuid AND a.archived_at IS NULL
		  AND al.person_id = ANY($2::uuid[])
		GROUP BY al.person_id`,
		workspaceID, pq.Array(personIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var personID string
		var lastAt time.Time
		if err := rows.Scan(&personID, &lastAt); err != nil {
			return nil, err
		}
		out[personID] = lastAt
	}
	return out, rows.Err()
}

// attachLastActivity mutates people in place, setting LastActivityAt.
func (s *PersonStore) attachLastActivity(ctx context.Context, tx *sql.Tx, workspaceID string, people []*domain.Person) error {
	personIDs := make([]string, len(people))
	for i, p := range people {
		personIDs[i] = p.ID
	}
	byPerson, err := s.lastActivityFor(ctx, tx, workspaceID, personIDs)
	if err != nil {
		return err
	}
	for _, p := range people {
		if t, ok := byPerson[p.ID]; ok {
			t := t
			p.LastActivityAt = &t
		}
	}
	return nil
}

// attachStrength computes PO-F-3 for each person and mutates the pointed-to
// slice elements so the caller sees the attached score.
func (s *PersonStore) attachStrength(ctx context.Context, tx *sql.Tx, workspaceID string, people []*domain.Person) error {
	personIDs := make([]string, len(people))
	for i, p := range people {
		personIDs[i] = p.ID
	}
	byPerson, err := s.strengthActivitiesFor(ctx, tx, workspaceID, personIDs)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, p := range people {
		result := strength.ComputeStrength(now, byPerson[p.ID])
		p.Strength = domain.PersonStrengthFrom(result)
	}
	return nil
}

// StrengthBreakdown returns PO-F-3's full evidence for one person: the
// literal factor values plus the contributing activities, for the
// non-black-box evidence read (PO-EXT-2). ErrNotFound if the person doesn't
// exist or is archived (mirrors Get).
func (s *PersonStore) StrengthBreakdown(ctx context.Context, id, workspaceID string) (strength.Result, error) {
	if _, err := s.Get(ctx, id, workspaceID); err != nil {
		return strength.Result{}, err
	}
	var result strength.Result
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		byPerson, err := s.strengthActivitiesFor(ctx, tx, workspaceID, []string{id})
		if err != nil {
			return err
		}
		result = strength.ComputeStrength(time.Now().UTC(), byPerson[id])
		return nil
	})
	return result, err
}
