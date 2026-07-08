// Package records implements the records-depth read side: the hierarchy roll-up
// (RD-FORM-1) that aggregates three RD-PARAM-2 measures over an organization's parent tree.
package records

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
	"github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// RollupStore computes GET /organizations/{id}/hierarchy-rollup (RD-FORM-1) over the
// organization.parent_org_id self-FK.
type RollupStore struct{ db *sql.DB }

// NewRollupStore returns a RollupStore backed by db.
func NewRollupStore(db *sql.DB) *RollupStore { return &RollupStore{db: db} }

// RestrictedNode is a descendant the viewer's row_scope cannot read, disclosed in the roll-up
// (RD-AC-1) rather than silently summed.
type RestrictedNode struct {
	ID          string
	DisplayName string
}

// RollupResult is the computed hierarchy roll-up for one root organization.
type RollupResult struct {
	RootID                 string
	Scope                  string // "tree" | "self"
	WeightedPipelineMinor  int64
	ClosedWonMinor         int64
	BaseCurrency           string
	ActivityCount30d       int
	AggregatedAccountCount int
	RestrictedExcluded     []RestrictedNode
	ComputedAt             time.Time
}

// treeNode is one row of the bounded recursive-CTE walk over organization.parent_org_id.
type treeNode struct {
	id       string
	parentID sql.NullString
	name     string
	ownerID  sql.NullString
}

// currentQuarterBounds returns the [start, end) calendar-quarter window containing now, expressed
// in loc (DM-TZ-4 calendar-aligned reporting periods). start is inclusive, end exclusive.
func currentQuarterBounds(now time.Time, loc *time.Location) (start, end time.Time) {
	n := now.In(loc)
	qStartMonth := ((int(n.Month())-1)/3)*3 + 1
	start = time.Date(n.Year(), time.Month(qStartMonth), 1, 0, 0, 0, 0, loc)
	end = start.AddDate(0, 3, 0)
	return start, end
}

// nodeReadable applies the RD-T04 row_scope readability table: "all" always passes; "own"
// requires ownerIsViewer or hasGrant (NULL owner is not auto-included); "team" also passes
// if ownerIsTeammate. Any other scope returns false.
func nodeReadable(rowScope string, ownerID, viewerID sql.NullString, teammates map[string]bool, hasGrant bool) bool {
	switch rowScope {
	case "all":
		return true
	case "own":
		return hasGrant || ownerIsViewer(ownerID, viewerID)
	case "team":
		return hasGrant || ownerIsViewer(ownerID, viewerID) || ownerIsTeammate(ownerID, teammates)
	default:
		return false
	}
}

func ownerIsViewer(ownerID, viewerID sql.NullString) bool {
	return ownerID.Valid && viewerID.Valid && ownerID.String == viewerID.String
}

func ownerIsTeammate(ownerID sql.NullString, teammates map[string]bool) bool {
	return ownerID.Valid && teammates[ownerID.String]
}

// sumMinor totals a slice of minor-unit contributions. An empty/nil slice yields a real 0
// (RD-FORM-1: a node with no contributing rows contributes 0, never an omitted field).
func sumMinor(vals []int64) int64 {
	var total int64
	for _, v := range vals {
		total += v
	}
	return total
}

// Compute returns the roll-up for rootID under the viewer (workspaceID, userID) and scope
// ("tree" | "self"). It returns errs.ErrNotFound when rootID does not exist, is out-of-workspace,
// is archived, or (tree scope) is unreadable under the viewer's row_scope. A missing stored FX
// rate for a needed pair is returned as an unwrapped *deals.FXRateUnavailableError.
func (s *RollupStore) Compute(ctx context.Context, rootID, workspaceID, userID, scope string) (RollupResult, error) {
	// row_scope is loaded on the pool (mirrors auth.RbacMiddleware); "self" scope never needs it.
	var rowScope string
	if scope != "self" {
		perms, err := auth.LoadRolePermissions(ctx, s.db, workspaceID, userID)
		if err != nil {
			return RollupResult{}, err
		}
		rowScope = perms["organization"].Actions["read"].RowScope
	}

	out := RollupResult{RootID: rootID, Scope: scope, RestrictedExcluded: []RestrictedNode{}}
	asOf := time.Now().UTC()

	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		baseCurrency, loc, err := s.loadWorkspaceMeta(ctx, tx, workspaceID)
		if err != nil {
			return err
		}
		out.BaseCurrency = baseCurrency

		nodes, err := s.loadTree(ctx, tx, rootID, workspaceID)
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			return errs.ErrNotFound
		}

		var includedIDs []string
		var restricted []RestrictedNode
		if scope == "self" {
			includedIDs = []string{rootID}
		} else {
			includedIDs, restricted, err = s.resolveReadable(ctx, tx, rootID, workspaceID, userID, rowScope, nodes)
			if err != nil {
				return err
			}
		}
		out.RestrictedExcluded = restricted
		out.AggregatedAccountCount = len(includedIDs)

		start, end := currentQuarterBounds(asOf, loc)

		weighted, err := s.weightedPipeline(ctx, tx, workspaceID, baseCurrency, asOf, includedIDs)
		if err != nil {
			return err
		}
		out.WeightedPipelineMinor = weighted

		closedWon, err := s.closedWon(ctx, tx, workspaceID, includedIDs, start, end)
		if err != nil {
			return err
		}
		out.ClosedWonMinor = closedWon

		activityCount, err := s.activityCount30d(ctx, tx, workspaceID, includedIDs, asOf)
		if err != nil {
			return err
		}
		out.ActivityCount30d = activityCount
		return nil
	})
	if err != nil {
		return RollupResult{}, err
	}
	out.ComputedAt = asOf
	return out, nil
}

// loadWorkspaceMeta returns the workspace's base currency and reporting timezone. An unparseable
// timezone degrades to UTC rather than failing the read.
func (s *RollupStore) loadWorkspaceMeta(ctx context.Context, tx *sql.Tx, workspaceID string) (string, *time.Location, error) {
	var baseCurrency, timezone string
	if err := tx.QueryRowContext(ctx, `
		SELECT base_currency, timezone
		FROM workspace
		WHERE id=$1::uuid`,
		workspaceID).Scan(&baseCurrency, &timezone); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, errs.ErrNotFound
		}
		return "", nil, err
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return baseCurrency, loc, nil
}

// loadTree walks organization.parent_org_id from rootID via a bounded recursive CTE (depth-capped
// at 50 as a defensive belt over the existing cycle-prevention trigger) and returns every live,
// in-workspace node in the subtree, root first.
func (s *RollupStore) loadTree(ctx context.Context, tx *sql.Tx, rootID, workspaceID string) ([]treeNode, error) {
	rows, err := tx.QueryContext(ctx, `
		WITH RECURSIVE tree AS (
			SELECT id, parent_org_id, name, owner_id, 0 AS depth
			FROM organization
			WHERE id = $1::uuid AND workspace_id = $2::uuid AND archived_at IS NULL
			UNION ALL
			SELECT o.id, o.parent_org_id, o.name, o.owner_id, t.depth + 1
			FROM organization o
			JOIN tree t ON o.parent_org_id = t.id
			WHERE o.workspace_id = $2::uuid AND o.archived_at IS NULL AND t.depth < 50
		)
		SELECT id, parent_org_id, name, owner_id FROM tree ORDER BY depth, id`,
		rootID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	nodes := make([]treeNode, 0, 16)
	for rows.Next() {
		var n treeNode
		if err := rows.Scan(&n.id, &n.parentID, &n.name, &n.ownerID); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// resolveReadable applies the row_scope readability rules top-down: it BFS-walks from the root
// over the parent→children adjacency, including readable nodes and, at the first unreadable node
// on a branch, recording it in restrictedExcluded and NOT descending into its subtree
// (RD-AC-8's decomposition boundary). Returns errs.ErrNotFound when the root itself is unreadable.
func (s *RollupStore) resolveReadable(ctx context.Context, tx *sql.Tx, rootID, workspaceID, userID, rowScope string, nodes []treeNode) ([]string, []RestrictedNode, error) {
	byID := make(map[string]treeNode, len(nodes))
	children := make(map[string][]string, len(nodes))
	allIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		byID[n.id] = n
		allIDs = append(allIDs, n.id)
		if n.parentID.Valid {
			children[n.parentID.String] = append(children[n.parentID.String], n.id)
		}
	}

	viewerID := sql.NullString{String: userID, Valid: userID != ""}
	var teammates map[string]bool
	var grants map[string]bool
	if rowScope != "all" {
		var err error
		teammates, err = s.loadTeammates(ctx, tx, workspaceID, userID)
		if err != nil {
			return nil, nil, err
		}
		grants, err = s.loadGrants(ctx, tx, workspaceID, userID, allIDs)
		if err != nil {
			return nil, nil, err
		}
	}

	readable := func(id string) bool {
		n := byID[id]
		return nodeReadable(rowScope, n.ownerID, viewerID, teammates, grants[id])
	}

	if !readable(rootID) {
		return nil, nil, errs.ErrNotFound
	}

	included := make([]string, 0, len(nodes))
	restricted := []RestrictedNode{}
	queue := []string{rootID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		included = append(included, id)
		for _, childID := range children[id] {
			if readable(childID) {
				queue = append(queue, childID)
				continue
			}
			restricted = append(restricted, RestrictedNode{ID: childID, DisplayName: byID[childID].name})
		}
	}
	return included, restricted, nil
}

// loadTeammates returns the set of user ids sharing at least one team with userID (the set
// includes userID itself via the self-join).
func (s *RollupStore) loadTeammates(ctx context.Context, tx *sql.Tx, workspaceID, userID string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT tm2.user_id
		FROM team_membership tm1
		JOIN team_membership tm2 ON tm2.team_id = tm1.team_id AND tm2.workspace_id = tm1.workspace_id
		WHERE tm1.workspace_id = $1::uuid AND tm1.user_id = $2::uuid`,
		workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// loadGrants returns the set of tree-node ids for which the viewer holds a live record_grant
// (organization, read|write, unexpired), matched either directly by user or via one of the
// viewer's teams.
func (s *RollupStore) loadGrants(ctx context.Context, tx *sql.Tx, workspaceID, userID string, nodeIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(nodeIDs) == 0 {
		return out, nil
	}
	teamIDs, err := s.loadTeamIDs(ctx, tx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT rg.record_id
		FROM record_grant rg
		WHERE rg.workspace_id = $1::uuid
		  AND rg.record_type = 'organization'
		  AND rg.record_id = ANY($2::uuid[])
		  AND rg.access IN ('read','write')
		  AND (rg.expires_at IS NULL OR rg.expires_at > now())
		  AND (
		        (rg.subject_type = 'user' AND rg.subject_id = $3::uuid)
		     OR (rg.subject_type = 'team' AND rg.subject_id = ANY($4::uuid[]))
		  )`,
		workspaceID, pq.Array(nodeIDs), userID, pq.Array(teamIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// loadTeamIDs returns the ids of the teams the viewer belongs to.
func (s *RollupStore) loadTeamIDs(ctx context.Context, tx *sql.Tx, workspaceID, userID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT team_id
		FROM team_membership
		WHERE workspace_id = $1::uuid AND user_id = $2::uuid`,
		workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]string, 0, 8)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// weightedPipeline sums DEAL-FORM-2's per-deal weighted open-pipeline value over every included
// node, mirroring deals/adapters/store_rollup.go: AsOfFXRate → ConvertToBase → WeightedValue.
// A missing stored FX rate fails the whole read (never a rate-of-1 fallback). Rate lookups are
// memoized per source currency within the call (asOf and base currency are fixed).
func (s *RollupStore) weightedPipeline(ctx context.Context, tx *sql.Tx, workspaceID, baseCurrency string, asOf time.Time, includedIDs []string) (int64, error) {
	if len(includedIDs) == 0 {
		return 0, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT d.amount_minor, d.currency, s.win_probability
		FROM deal d
		JOIN stage s
		  ON s.id = d.stage_id
		 AND s.workspace_id = d.workspace_id
		 AND s.archived_at IS NULL
		WHERE d.workspace_id = $1::uuid
		  AND d.organization_id = ANY($2::uuid[])
		  AND d.status = 'open'
		  AND d.archived_at IS NULL`,
		workspaceID, pq.Array(includedIDs))
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	type openDeal struct {
		amountMinor    sql.NullInt64
		currency       sql.NullString
		winProbability int
	}
	var openDeals []openDeal
	for rows.Next() {
		var d openDeal
		if err := rows.Scan(&d.amountMinor, &d.currency, &d.winProbability); err != nil {
			return 0, err
		}
		openDeals = append(openDeals, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	rateCache := map[string]float64{}
	weighted := make([]int64, 0, len(openDeals))
	for _, d := range openDeals {
		if !d.amountMinor.Valid {
			continue // no amount contributes 0, mirroring the open-pipeline rollup's NoAmount case
		}
		baseValue := d.amountMinor.Int64
		if d.currency.Valid && d.currency.String != "" && d.currency.String != baseCurrency {
			rate, ok := rateCache[d.currency.String]
			if !ok {
				rate, err = deals.AsOfFXRate(ctx, tx, workspaceID, d.currency.String, baseCurrency, asOf)
				if err != nil {
					return 0, err
				}
				rateCache[d.currency.String] = rate
			}
			baseValue = deals.ConvertToBase(d.amountMinor.Int64, d.currency.String, baseCurrency, rate)
		}
		weighted = append(weighted, deals.WeightedValue(baseValue, d.winProbability))
	}
	return sumMinor(weighted), nil
}

// closedWon sums deal.amount_minor_base (the frozen-rate GENERATED column, migration 000075) over
// won deals whose closed_at falls inside the [start, end) current-quarter window, for every
// included node. No AsOfFXRate call is needed and no 422 can arise from this measure.
func (s *RollupStore) closedWon(ctx context.Context, tx *sql.Tx, workspaceID string, includedIDs []string, start, end time.Time) (int64, error) {
	if len(includedIDs) == 0 {
		return 0, nil
	}
	var total int64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(d.amount_minor_base), 0)
		FROM deal d
		WHERE d.workspace_id = $1::uuid
		  AND d.organization_id = ANY($2::uuid[])
		  AND d.status = 'won'
		  AND d.archived_at IS NULL
		  AND d.closed_at >= $3 AND d.closed_at < $4`,
		workspaceID, pq.Array(includedIDs), start, end).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// activityCount30d counts the distinct activities linked to any included node
// (activity_link.entity_type='organization', DM-CONV-17) with occurred_at within the last 30 days
// and not archived.
func (s *RollupStore) activityCount30d(ctx context.Context, tx *sql.Tx, workspaceID string, includedIDs []string, asOf time.Time) (int, error) {
	if len(includedIDs) == 0 {
		return 0, nil
	}
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT a.id)
		FROM activity a
		JOIN activity_link al
		  ON al.activity_id = a.id
		 AND al.workspace_id = a.workspace_id
		WHERE a.workspace_id = $1::uuid
		  AND al.entity_type = 'organization'
		  AND al.organization_id = ANY($2::uuid[])
		  AND a.occurred_at >= $3::timestamptz - interval '30 days'
		  AND a.archived_at IS NULL`,
		workspaceID, pq.Array(includedIDs), asOf).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Quota is a per-owner or per-team revenue target for one period (RD-DDL-2).
type Quota = adapters.Quota

// QuotaListFilter narrows a List call to a specific owner or team.
type QuotaListFilter = adapters.QuotaListFilter

// QuotaStore executes parameterized SQL against the quota table.
type QuotaStore = adapters.QuotaStore

// ErrOwnerXorTeamRequired fires when owner_id XOR team_id is not satisfied (RD-DDL-2).
var ErrOwnerXorTeamRequired = adapters.ErrOwnerXorTeamRequired

// NewQuotaStore returns a QuotaStore backed by db.
func NewQuotaStore(db *sql.DB) *QuotaStore { return adapters.NewQuotaStore(db) }
