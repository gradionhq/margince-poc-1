package crmcore

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// OrgListFilter holds optional predicates for List. Zero value = no extra filters.
type OrgListFilter struct {
	Classification string
	RelevanceGTE   *int
	Domain         string
	OwnerID        string
}

func buildOrgListWhere(f OrgListFilter, args []any, n int) (string, []any) {
	where := ""
	if f.Classification != "" {
		n++
		args = append(args, f.Classification)
		where += fmt.Sprintf(` AND classification=$%d`, n)
	}
	if f.RelevanceGTE != nil {
		n++
		args = append(args, *f.RelevanceGTE)
		where += fmt.Sprintf(` AND relevance >= $%d`, n)
	}
	if f.OwnerID != "" {
		n++
		args = append(args, f.OwnerID)
		where += fmt.Sprintf(` AND owner_id=$%d::uuid`, n)
	}
	if domain := strings.ToLower(strings.TrimSpace(f.Domain)); domain != "" {
		n++
		args = append(args, domain)
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM organization_domain od
			WHERE od.organization_id=organization.id AND od.workspace_id=organization.workspace_id
			  AND od.domain=$%d AND od.archived_at IS NULL)`, n)
	}
	return where, args
}

// List returns a page of live organizations; sort="" or "id" uses ID keyset cursor, and
// "strength"/"-strength" fetches all matching rows, attaches aggregates, sorts by score,
// offset-paginates.
func (s *OrgStore) List(ctx context.Context, workspaceID, cursor string, limit int, sortVal string, filter OrgListFilter) ([]Organization, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	switch sortVal {
	case "strength":
		return s.listByOrgStrength(ctx, workspaceID, cursor, limit, false, filter)
	case "-strength":
		return s.listByOrgStrength(ctx, workspaceID, cursor, limit, true, filter)
	default:
		return s.listByOrgID(ctx, workspaceID, cursor, limit, filter)
	}
}

func (s *OrgStore) listByOrgID(ctx context.Context, workspaceID, cursor string, limit int, filter OrgListFilter) ([]Organization, string, error) {
	out := []Organization{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID, cursor, limit + 1}
		extraWhere, args := buildOrgListWhere(filter, args, 3)
		//nolint:gosec // G202: extraWhere injects only bound-param indices ($N); all filter values are passed via args
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social, version, source, captured_by, created_at, updated_at
			FROM organization
			WHERE workspace_id=$1::uuid AND archived_at IS NULL
			  AND ($2 = '' OR id::text > $2)`+extraWhere+`
			ORDER BY id LIMIT $3`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var o Organization
			var socialRaw []byte
			if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw, &o.Version, &o.Source, &o.CapturedBy,
				&o.CreatedAt, &o.UpdatedAt); err != nil {
				return err
			}
			o.Social = map[string]any{}
			unmarshalJSON(socialRaw, &o.Social)
			out = append(out, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*Organization, len(out))
		for i := range out {
			ptrs[i] = &out[i]
		}
		return attachOrgAggregates(ctx, tx, s.personStore.strengthActivitiesFor, workspaceID, ptrs)
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

func (s *OrgStore) listByOrgStrength(ctx context.Context, workspaceID, cursor string, limit int, descending bool, filter OrgListFilter) ([]Organization, string, error) {
	offset := decodeOffsetCursor(cursor)
	all := []Organization{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID}
		extraWhere, args := buildOrgListWhere(filter, args, 1)
		//nolint:gosec // G202: extraWhere injects only bound-param indices ($N); all filter values are passed via args
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social, version, source, captured_by, created_at, updated_at
			FROM organization
			WHERE workspace_id=$1::uuid AND archived_at IS NULL`+extraWhere+`
			ORDER BY id`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var o Organization
			var socialRaw []byte
			if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw, &o.Version, &o.Source, &o.CapturedBy,
				&o.CreatedAt, &o.UpdatedAt); err != nil {
				return err
			}
			o.Social = map[string]any{}
			unmarshalJSON(socialRaw, &o.Social)
			all = append(all, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*Organization, len(all))
		for i := range all {
			ptrs[i] = &all[i]
		}
		return attachOrgAggregates(ctx, tx, s.personStore.strengthActivitiesFor, workspaceID, ptrs)
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
			if descending {
				return si.Score > sj.Score
			}
			return si.Score < sj.Score
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
