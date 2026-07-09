// Package adapters — OrgStore.List (organizations module, WS-E-a).
// Ported from modules/directory/store_org_list.go (package crmcore → package adapters).
package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

func buildOrgListWhere(f domain.OrgListFilter, args []any, n int) (string, []any) {
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
		ownerArg := n
		n++
		args = append(args, f.OwnerID)
		where += fmt.Sprintf(` AND (owner_id=$%d::uuid OR EXISTS (
			SELECT 1 FROM record_grant rg
			WHERE rg.workspace_id = organization.workspace_id AND rg.record_type = 'organization' AND rg.record_id = organization.id
			  AND rg.subject_type = 'user' AND rg.subject_id = $%d::uuid
			  AND (rg.expires_at IS NULL OR rg.expires_at > now())))`, ownerArg, n)
	}
	for name, value := range f.CustomFilters {
		n++
		args = append(args, value)
		where += fmt.Sprintf(` AND %s::text = $%d`, pq.QuoteIdentifier(name), n)
	}
	if domainVal := strings.ToLower(strings.TrimSpace(f.Domain)); domainVal != "" {
		n++
		args = append(args, domainVal)
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
func (s *OrgStore) List(ctx context.Context, workspaceID, cursor string, limit int, sortVal string, filter domain.OrgListFilter) ([]domain.Organization, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	active, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "organization")
	if err != nil {
		return nil, "", err
	}
	switch sortVal {
	case "strength":
		return s.listByOrgStrength(ctx, workspaceID, cursor, limit, false, filter, active)
	case "-strength":
		return s.listByOrgStrength(ctx, workspaceID, cursor, limit, true, filter, active)
	case "", "id":
		return s.listByOrgID(ctx, workspaceID, cursor, limit, filter, active)
	default:
		key := strings.TrimPrefix(sortVal, "-")
		for _, c := range active {
			if c.ColumnName == key {
				return s.listByCustomColumn(ctx, workspaceID, cursor, limit, sortVal, filter, active)
			}
		}
		return s.listByOrgID(ctx, workspaceID, cursor, limit, filter, active)
	}
}

func (s *OrgStore) listByCustomColumn(ctx context.Context, workspaceID, cursor string, limit int, sortVal string, filter domain.OrgListFilter, active []customfields.Column) ([]domain.Organization, string, error) {
	col := strings.TrimPrefix(sortVal, "-")
	desc := strings.HasPrefix(sortVal, "-")
	offset := sqlutil.DecodeOffsetCursor(cursor)
	all := []domain.Organization{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID}
		extraWhere, args := buildOrgListWhere(filter, args, 1)
		//nolint:gosec // G202: cfSelectSuffix + pq.QuoteIdentifier(col) inject only catalog-derived cf_* column names; extraWhere injects only $N placeholders, all values bound via args
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social`+cfSelectSuffix(active)+`, version, source, captured_by, created_at, updated_at
			FROM organization
			WHERE workspace_id=$1::uuid AND archived_at IS NULL`+extraWhere+`
			ORDER BY `+pq.QuoteIdentifier(col)+` `+func() string {
			if desc {
				return "DESC NULLS LAST"
			}
			return "ASC NULLS LAST"
		}()+`, id`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var o domain.Organization
			var socialRaw []byte
			dests := customfields.ScanDests(active)
			scanArgs := append([]any{
				&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw,
			}, dests...)
			scanArgs = append(scanArgs, &o.Version, &o.Source, &o.CapturedBy, &o.CreatedAt, &o.UpdatedAt)
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			o.Social = map[string]any{}
			sqlutil.UnmarshalJSON(socialRaw, &o.Social)
			o.CustomFields = customfields.ExtractValues(active, dests)
			all = append(all, o)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}
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

func (s *OrgStore) listByOrgID(ctx context.Context, workspaceID, cursor string, limit int, filter domain.OrgListFilter, active []customfields.Column) ([]domain.Organization, string, error) {
	out := []domain.Organization{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID, cursor, limit + 1}
		extraWhere, args := buildOrgListWhere(filter, args, 3)
		//nolint:gosec // G202: cfSelectSuffix injects only pq.QuoteIdentifier'd cf_* names; extraWhere injects only bound-param indices ($N); all filter values are passed via args
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social`+cfSelectSuffix(active)+`, version, source, captured_by, created_at, updated_at
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
			var o domain.Organization
			var socialRaw []byte
			dests := customfields.ScanDests(active)
			scanArgs := append([]any{
				&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw,
			}, dests...)
			scanArgs = append(scanArgs, &o.Version, &o.Source, &o.CapturedBy, &o.CreatedAt, &o.UpdatedAt)
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			o.Social = map[string]any{}
			sqlutil.UnmarshalJSON(socialRaw, &o.Social)
			o.CustomFields = customfields.ExtractValues(active, dests)
			out = append(out, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*domain.Organization, len(out))
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

func (s *OrgStore) listByOrgStrength(ctx context.Context, workspaceID, cursor string, limit int, descending bool, filter domain.OrgListFilter, active []customfields.Column) ([]domain.Organization, string, error) {
	offset := sqlutil.DecodeOffsetCursor(cursor)
	all := []domain.Organization{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		args := []any{workspaceID}
		extraWhere, args := buildOrgListWhere(filter, args, 1)
		//nolint:gosec // G202: cfSelectSuffix injects only pq.QuoteIdentifier'd cf_* names; extraWhere injects only bound-param indices ($N); all filter values are passed via args
		rows, err := tx.QueryContext(ctx, `
			SELECT id, workspace_id, name, website, classification, relevance,
			       owner_id, social`+cfSelectSuffix(active)+`, version, source, captured_by, created_at, updated_at
			FROM organization
			WHERE workspace_id=$1::uuid AND archived_at IS NULL`+extraWhere+`
			ORDER BY id`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var o domain.Organization
			var socialRaw []byte
			dests := customfields.ScanDests(active)
			scanArgs := append([]any{
				&o.ID, &o.WorkspaceID, &o.DisplayName, &o.Website, &o.Classification, &o.Relevance,
				&o.OwnerID, &socialRaw,
			}, dests...)
			scanArgs = append(scanArgs, &o.Version, &o.Source, &o.CapturedBy, &o.CreatedAt, &o.UpdatedAt)
			if err := rows.Scan(scanArgs...); err != nil {
				return err
			}
			o.Social = map[string]any{}
			sqlutil.UnmarshalJSON(socialRaw, &o.Social)
			o.CustomFields = customfields.ExtractValues(active, dests)
			all = append(all, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		ptrs := make([]*domain.Organization, len(all))
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
		next = sqlutil.EncodeOffsetCursor(end)
	} else {
		end = len(all)
	}
	return all[offset:end], next, nil
}
