// Package adapters — org-strength aggregation (organizations module, WS-E-a).
// Ported from modules/directory/store_org_strength.go (package crmcore → package adapters).
// Activity, ComputeStrength, Bucket come from the shared
// github.com/gradionhq/margince/backend/internal/shared/kernel/strength package.
package adapters

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/strength"
)

func orgContactCounts(ctx context.Context, tx *sql.Tx, workspaceID string, orgIDs []string) (map[string]int, error) {
	out := map[string]int{}
	if len(orgIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT organization_id, COUNT(DISTINCT person_id)
		FROM relationship
		WHERE workspace_id=$1::uuid AND kind='employment' AND archived_at IS NULL
		  AND organization_id = ANY($2::uuid[])
		GROUP BY organization_id`,
		workspaceID, pq.Array(orgIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var orgID string
		var n int
		if err := rows.Scan(&orgID, &n); err != nil {
			return nil, err
		}
		out[orgID] = n
	}
	return out, rows.Err()
}

func orgOpenDealCounts(ctx context.Context, tx *sql.Tx, workspaceID string, orgIDs []string) (map[string]int, error) {
	out := map[string]int{}
	if len(orgIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT organization_id, COUNT(*)
		FROM deal
		WHERE workspace_id=$1::uuid AND status='open' AND archived_at IS NULL
		  AND organization_id = ANY($2::uuid[])
		GROUP BY organization_id`,
		workspaceID, pq.Array(orgIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var orgID string
		var n int
		if err := rows.Scan(&orgID, &n); err != nil {
			return nil, err
		}
		out[orgID] = n
	}
	return out, rows.Err()
}

func orgEmployeeIDs(ctx context.Context, tx *sql.Tx, workspaceID string, orgIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(orgIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT organization_id, person_id
		FROM relationship
		WHERE workspace_id=$1::uuid AND kind='employment' AND archived_at IS NULL
		  AND organization_id = ANY($2::uuid[])`,
		workspaceID, pq.Array(orgIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var orgID, personID string
		if err := rows.Scan(&orgID, &personID); err != nil {
			return nil, err
		}
		out[orgID] = append(out[orgID], personID)
	}
	return out, rows.Err()
}

func personDisplayNames(ctx context.Context, tx *sql.Tx, personIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(personIDs) == 0 {
		return out, nil
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, full_name FROM person WHERE id = ANY($1::uuid[])`,
		pq.Array(personIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// attachOrgAggregates mutates orgs in place: contact_count, open_deal_count,
// org_strength. Wires domain.OrgStrength() unmodified — never reimplements the formula.
//
//nolint:cyclop // org aggregate assembly: one branch per query + per-person loop + nil guard; the branches are inherently sequential IO steps, not reducible without hiding the data-flow
func attachOrgAggregates(
	ctx context.Context,
	tx *sql.Tx,
	strengthActivities func(context.Context, *sql.Tx, string, []string) (map[string][]strength.Activity, error),
	workspaceID string,
	orgs []*domain.Organization,
) error {
	orgIDs := make([]string, len(orgs))
	for i, o := range orgs {
		orgIDs[i] = o.ID
	}
	contactCounts, err := orgContactCounts(ctx, tx, workspaceID, orgIDs)
	if err != nil {
		return err
	}
	openDealCounts, err := orgOpenDealCounts(ctx, tx, workspaceID, orgIDs)
	if err != nil {
		return err
	}
	employeesByOrg, err := orgEmployeeIDs(ctx, tx, workspaceID, orgIDs)
	if err != nil {
		return err
	}
	var allPersonIDs []string
	seen := map[string]bool{}
	for _, pids := range employeesByOrg {
		for _, id := range pids {
			if !seen[id] {
				seen[id] = true
				allPersonIDs = append(allPersonIDs, id)
			}
		}
	}
	activitiesByPerson, err := strengthActivities(ctx, tx, workspaceID, allPersonIDs)
	if err != nil {
		return err
	}
	names, err := personDisplayNames(ctx, tx, allPersonIDs)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, o := range orgs {
		o.ContactCount = contactCounts[o.ID]
		o.OpenDealCount = openDealCounts[o.ID]
		var inputs []domain.OrgStrengthInput
		for _, personID := range employeesByOrg[o.ID] {
			result := strength.ComputeStrength(now, activitiesByPerson[personID])
			if result.NoSignalYet {
				continue
			}
			var lastInteraction time.Time
			for _, a := range activitiesByPerson[personID] {
				if a.OccurredAt.After(lastInteraction) {
					lastInteraction = a.OccurredAt
				}
			}
			r := result
			inputs = append(inputs, domain.OrgStrengthInput{PersonID: personID, Strength: &r, LastInteraction: lastInteraction})
		}
		score, topPersonID, hasSignal := domain.OrgStrength(inputs)
		if !hasSignal {
			o.Strength = nil
			continue
		}
		o.Strength = &domain.OrgStrengthBlock{
			Score:         score,
			Bucket:        strength.Bucket(score),
			TopPersonID:   topPersonID,
			TopPersonName: names[topPersonID],
		}
	}
	return nil
}
