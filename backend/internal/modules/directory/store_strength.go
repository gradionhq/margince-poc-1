package crmcore

import (
	"context"
	"database/sql"
	"encoding/base64"
	"sort"
	"strconv"
	"time"

	"github.com/lib/pq"
)

// ---------------------------------------------------------------------------
// PersonStore — PO-F-3 relationship strength
// ---------------------------------------------------------------------------

// encodeOffsetCursor/decodeOffsetCursor page an in-memory-sorted list.
func encodeOffsetCursor(n int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(n)))
}

func decodeOffsetCursor(cursor string) (int, bool) {
	if cursor == "" {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func (s *PersonStore) listByStrength(ctx context.Context, workspaceID, cursor string, limit int, ascending bool) ([]Person, string, error) {
	offset, _ := decodeOffsetCursor(cursor)
	// Non-nil so an empty result marshals to a JSON array ([]), never null.
	all := []Person{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, full_name, first_name, last_name, title,
			       owner_id, social, version, source, captured_by, created_at, updated_at
			FROM person
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			ORDER BY id`,
			workspaceID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var p Person
			var socialRaw []byte
			if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.FullName, &p.FirstName, &p.LastName, &p.Title,
				&p.OwnerID, &socialRaw, &p.Version, &p.Source, &p.CapturedBy,
				&p.CreatedAt, &p.UpdatedAt); err != nil {
				return err
			}
			p.Social = map[string]any{}
			unmarshalJSON(socialRaw, &p.Social)
			all = append(all, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*Person, len(all))
		for i := range all {
			ptrs[i] = &all[i]
		}
		return s.attachStrength(ctx, tx, workspaceID, ptrs)
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
		next = encodeOffsetCursor(end)
	} else {
		end = len(all)
	}
	return all[offset:end], next, nil
}

// strengthActivitiesFor batch-fetches every live email/call/meeting activity
// linked to any of personIDs, grouped by person_id.
func (s *PersonStore) strengthActivitiesFor(ctx context.Context, tx *sql.Tx, workspaceID string, personIDs []string) (map[string][]StrengthActivity, error) {
	out := map[string][]StrengthActivity{}
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
		var a StrengthActivity
		if err := rows.Scan(&personID, &a.ID, &a.Kind, &a.Subject, &a.OccurredAt, &a.Direction); err != nil {
			return nil, err
		}
		out[personID] = append(out[personID], a)
	}
	return out, rows.Err()
}

// attachStrength computes PO-F-3 for each person and mutates the pointed-to
// slice elements so the caller sees the attached score.
func (s *PersonStore) attachStrength(ctx context.Context, tx *sql.Tx, workspaceID string, people []*Person) error {
	ids := make([]string, len(people))
	for i, p := range people {
		ids[i] = p.ID
	}
	byPerson, err := s.strengthActivitiesFor(ctx, tx, workspaceID, ids)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, p := range people {
		result := ComputeStrength(now, byPerson[p.ID])
		p.Strength = personStrengthFrom(result)
	}
	return nil
}

// StrengthBreakdown returns PO-F-3's full evidence for one person: the
// literal factor values plus the contributing activities, for the
// non-black-box evidence read (PO-EXT-2). ErrNotFound if the person doesn't
// exist or is archived (mirrors Get).
func (s *PersonStore) StrengthBreakdown(ctx context.Context, id, workspaceID string) (StrengthResult, error) {
	if _, err := s.Get(ctx, id, workspaceID); err != nil {
		return StrengthResult{}, err
	}
	var result StrengthResult
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		byPerson, err := s.strengthActivitiesFor(ctx, tx, workspaceID, []string{id})
		if err != nil {
			return err
		}
		result = ComputeStrength(time.Now().UTC(), byPerson[id])
		return nil
	})
	return result, err
}
