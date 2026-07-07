package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals/domain"
)

// ---------------------------------------------------------------------------
// DealStore — filtered listing (domain.DealListFilter, ListFiltered, and helpers)
// ---------------------------------------------------------------------------

var dealSortColumns = map[string]bool{
	"created_at":          true,
	"updated_at":          true,
	"amount_minor":        true,
	"expected_close_date": true,
	"last_activity_at":    true,
}

func dealOrderBy(sort string) string {
	if sort == "" {
		return "ORDER BY id"
	}
	var clauses []string
	for _, f := range strings.Split(sort, ",") {
		f = strings.TrimSpace(f)
		dir := "ASC"
		col := f
		if strings.HasPrefix(f, "-") {
			dir = "DESC"
			col = f[1:]
		}
		if !dealSortColumns[col] {
			continue
		}
		clauses = append(clauses, col+" "+dir)
	}
	clauses = append(clauses, "id")
	return "ORDER BY " + strings.Join(clauses, ", ")
}

// List delegates to ListFiltered with a zero filter, preserving the existing signature.
func (s *DealStore) List(ctx context.Context, workspaceID, cursor string, limit int) ([]domain.Deal, string, error) {
	return s.ListFiltered(ctx, workspaceID, cursor, limit, domain.DealListFilter{})
}

// buildDealListWhereBasic appends the simple equality/identity filters
// (pipeline/stage/owner/organization/status) to where/args, returning the
// updated where clause, args, and next $N index.
func buildDealListWhereBasic(f domain.DealListFilter, where string, args []any, n int) (string, []any, int) {
	if f.PipelineID != "" {
		n++
		args = append(args, f.PipelineID)
		where += fmt.Sprintf(` AND pipeline_id=$%d::uuid`, n)
	}
	if f.StageID != "" {
		n++
		args = append(args, f.StageID)
		where += fmt.Sprintf(` AND stage_id=$%d::uuid`, n)
	}
	if f.OwnerID != "" {
		n++
		args = append(args, f.OwnerID)
		where += fmt.Sprintf(` AND owner_id=$%d::uuid`, n)
	}
	if f.OrganizationID != "" {
		n++
		args = append(args, f.OrganizationID)
		where += fmt.Sprintf(` AND organization_id=$%d::uuid`, n)
	}
	if f.Status != "" {
		n++
		args = append(args, f.Status)
		where += fmt.Sprintf(` AND status=$%d`, n)
	}
	return where, args, n
}

// buildDealListWhereExtra appends the remaining filters (stalled/forecast
// category/partner org/person) to where/args, returning the updated where
// clause, args, and next $N index.
func buildDealListWhereExtra(f domain.DealListFilter, where string, args []any, n int) (string, []any, int) {
	if f.Stalled {
		// IsStalled is only ever true for status='open' deals (DEAL-FORM-3), so this
		// literal is a safe, sound narrowing pre-filter — never excludes a true
		// positive. The exact stalled/suppressed decision (which SQL cannot express
		// without duplicating IsStalled) happens in Go in ListFiltered, on the fetched rows.
		where += ` AND status='open'`
	}
	if f.ForecastCategory != "" {
		n++
		args = append(args, f.ForecastCategory)
		where += fmt.Sprintf(` AND forecast_category=$%d`, n)
	}
	if f.PartnerOrgID != "" {
		n++
		args = append(args, f.PartnerOrgID)
		where += fmt.Sprintf(` AND partner_org_id=$%d::uuid`, n)
	}
	if f.PersonID != "" {
		n++
		args = append(args, f.PersonID)
		where += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM relationship WHERE relationship.deal_id=deal.id AND relationship.kind='deal_stakeholder' AND relationship.person_id=$%d::uuid AND relationship.archived_at IS NULL)`, n)
	}
	return where, args, n
}

// buildDealListWhere composes the full WHERE clause and bound args for
// ListFiltered from the fixed base predicate plus all optional filters in f.
func buildDealListWhere(workspaceID, cursor string, limit int, f domain.DealListFilter) (string, []any, int) {
	args := []any{workspaceID, cursor, limit + 1}
	n := 3 // next $N index

	where := `workspace_id=$1::uuid AND archived_at IS NULL AND ($2 = '' OR id::text > $2)`
	where, args, n = buildDealListWhereBasic(f, where, args, n)
	where, args, n = buildDealListWhereExtra(f, where, args, n)
	return where, args, n
}

// ListFiltered returns cursor-keyed, workspace-scoped deals matching f.
// Predicates are AND-ed; all filter values are bound params.
func (s *DealStore) ListFiltered(ctx context.Context, workspaceID, cursor string, limit int, f domain.DealListFilter) ([]domain.Deal, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	where, args, _ := buildDealListWhere(workspaceID, cursor, limit, f)

	out := []domain.Deal{}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		//nolint:gosec // G202: `where` injects only bound-param indices ($N), never user input; all filter values are passed via args
		rows, err := tx.QueryContext(ctx,
			`SELECT id, workspace_id, name, pipeline_id, stage_id,
			        organization_id, owner_id, partner_org_id,
			        amount_minor, currency, status, wait_until, last_activity_at,
			        version, source, captured_by, created_at, updated_at,
			        (SELECT max(occurred_at) FROM deal_stage_history WHERE deal_id=deal.id) AS stage_entered_at,
			        (SELECT count(*) FROM relationship WHERE deal_id=deal.id AND kind='deal_stakeholder' AND archived_at IS NULL) AS stakeholder_count
			 FROM deal
			 WHERE `+where+`
			 `+dealOrderBy(f.Sort)+` LIMIT $3`,
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var d domain.Deal
			var stageEnteredAt sql.NullTime
			if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Name, &d.PipelineID, &d.StageID,
				&d.OrganizationID, &d.OwnerID, &d.PartnerOrgID,
				&d.AmountMinor, &d.Currency, &d.Status, &d.WaitUntil, &d.LastActivityAt, &d.Version,
				&d.Source, &d.CapturedBy,
				&d.CreatedAt, &d.UpdatedAt,
				&stageEnteredAt, &d.StakeholderCount); err != nil {
				return err
			}
			if stageEnteredAt.Valid {
				d.StageEnteredAt = &stageEnteredAt.Time
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC()
	for i := range out {
		out[i].Stalled, _ = domain.IsStalled(out[i], now)
	}
	if f.Stalled {
		// See the SQL comment above: a limit+1 over-fetch of open deals can contain
		// non-stalled (suppressed) rows this trims below limit — an accepted
		// pagination simplification for this ticket's scope (see plan Global
		// Constraints).
		kept := out[:0]
		for _, d := range out {
			if d.Stalled {
				kept = append(kept, d)
			}
		}
		out = kept
	}

	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}
