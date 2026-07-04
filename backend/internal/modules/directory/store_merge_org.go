// Package crmcore — OrgStore.Merge (PO-AC-18, PO-AC-M1/M2/M6, org side).
// Mirrors PersonStore.Merge (store_merge_person.go) — see that file's
// doc-comment for the general merge-validation shape (self-merge,
// already-merged chain-following, optimistic concurrency). ErrSelfMerge,
// ErrAlreadyMerged, and ErrMergeTargetInvalid are declared once, in
// store_merge_person.go, and reused verbatim here.
package crmcore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// followOrgMergeChain walks merged_into_id until it finds an organization with
// no further merge pointer, returning that final id. Mirrors
// followMergeChain (store_merge_person.go) for the organization table.
func followOrgMergeChain(ctx context.Context, tx *sql.Tx, id, workspaceID string) (string, error) {
	current := id
	for {
		var next sql.NullString
		if err := tx.QueryRowContext(ctx,
			`SELECT merged_into_id::text FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			current, workspaceID).Scan(&next); err != nil {
			return "", err
		}
		if !next.Valid {
			return current, nil
		}
		current = next.String
	}
}

// validateOrgMergePair reads both loser and target rows inside tx, returning
// the loser's version + before-snapshot or a typed 422/404 error if the pair
// is not eligible for merge (already-merged loser, invalid target). Mirrors
// validateMergePair (store_merge_person.go) for the organization table.
//
//nolint:dupl // parallel per-entity merge validation: mirrors validateMergePair (store_merge_person.go) for organization; the SQL table names and error wiring differ by entity, a generic version would read worse than the explicit form
func validateOrgMergePair(ctx context.Context, tx *sql.Tx, loserID, targetID, workspaceID string) (mergeLoserState, error) {
	var state mergeLoserState
	var loserMergedInto sql.NullString
	var loserArchived sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT version, merged_into_id::text, archived_at, row_to_json(organization.*)
		FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		loserID, workspaceID).Scan(&state.version, &loserMergedInto, &loserArchived, &state.beforeRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, errs.ErrNotFound
		}
		return state, err
	}
	if loserMergedInto.Valid {
		survivor, err := followOrgMergeChain(ctx, tx, loserID, workspaceID)
		if err != nil {
			return state, err
		}
		return state, &ErrAlreadyMerged{SurvivorID: survivor}
	}

	var targetMergedInto sql.NullString
	var targetArchived sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`SELECT merged_into_id::text, archived_at FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		targetID, workspaceID).Scan(&targetMergedInto, &targetArchived); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, errs.ErrNotFound
		}
		return state, err
	}
	if targetMergedInto.Valid || targetArchived.Valid {
		survivor, err := followOrgMergeChain(ctx, tx, targetID, workspaceID)
		if err != nil {
			return state, err
		}
		return state, &ErrMergeTargetInvalid{SurvivorID: survivor}
	}
	return state, nil
}

// Merge relinks loserID's domains, deals (organization_id + partner_org_id),
// employment/partner relationships, activity links, and 1:1 partner row onto
// targetID, archives loserID with merged_into_id=targetID, and writes one
// audit_log row (action "merge", before=full pre-merge loser snapshot for
// reversibility) plus one organization.merged event_outbox row — all in a
// single workspace-scoped tx.
//
// FK enumeration (grep "REFERENCES organization(id)" backend/migrations/*.sql):
// organization_domain, deal.organization_id, deal.partner_org_id,
// activity_link.organization_id, relationship.organization_id,
// relationship.counterparty_org_id, and partner.organization_id are actively
// relinked because they describe the organization's *current* state.
// organization.parent_org_id describes a historical hierarchy fact about a
// specific org id and is deliberately left pointing at the archived loser —
// the row still exists (soft-archived, never deleted), so this is not an
// orphaned FK, just an unmoved historical record. TestOrgMergeFKWalkExhaustive
// proves this list is exhaustive against the live schema, not just this
// comment's memory of it.
func (s *OrgStore) Merge(ctx context.Context, loserID, targetID, workspaceID string) (Organization, error) {
	if loserID == targetID {
		return Organization{}, ErrSelfMerge
	}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		state, err := validateOrgMergePair(ctx, tx, loserID, targetID, workspaceID)
		if err != nil {
			return err
		}
		if err := relinkOrgFKs(ctx, tx, loserID, targetID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE organization SET merged_into_id=$3::uuid, archived_at=now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$4 AND merged_into_id IS NULL AND archived_at IS NULL`,
			loserID, workspaceID, targetID, state.version)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errs.ErrVersionSkew
		}
		e := crmaudit.EntryFromPrincipal(ctx, "merge", entityTypeOrganization, &loserID, json.RawMessage(state.beforeRaw), map[string]any{fieldMergedIntoID: targetID})
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("org merge audit: %w", err)
		}
		payload := marshalJSON(map[string]any{"organization_id": loserID, "merged_into_id": targetID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,'organization.merged',$2::uuid,$3)`,
			workspaceID, loserID, payload); err != nil {
			return fmt.Errorf("org merge event: %w", err)
		}
		return nil
	})
	if err != nil {
		return Organization{}, err
	}
	return s.Get(ctx, targetID, workspaceID)
}

// relinkOrgFKs moves loserID's domain/deal/relationship/activity_link/partner
// rows onto targetID, demoting (never deleting) any conflicting is_primary
// row on the SURVIVOR side per PO-AC-M1, and collapsing duplicate/1:1 rows
// (organization_domain, partner) instead of violating their unique
// constraints.
func relinkOrgFKs(ctx context.Context, tx *sql.Tx, loserID, targetID string) error {
	// organization_domain: uq_org_domain is UNIQUE(workspace_id, domain) —
	// a domain string can only ever point at one organization in the
	// workspace, so drop any loser domain that duplicates a live target
	// domain before moving the rest. If the moved row is_primary and the
	// target already has a primary domain, demote it (target's original
	// primary wins, PO-AC-M1; uq_org_domain_primary).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM organization_domain
		WHERE organization_id=$1::uuid AND archived_at IS NULL AND domain IN (
			SELECT domain FROM organization_domain WHERE organization_id=$2::uuid AND archived_at IS NULL)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink organization_domain dedupe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE organization_domain SET organization_id=$2::uuid,
		  is_primary = is_primary AND NOT EXISTS (
		    SELECT 1 FROM organization_domain WHERE organization_id=$2::uuid AND is_primary AND archived_at IS NULL)
		WHERE organization_id=$1::uuid AND archived_at IS NULL`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink organization_domain: %w", err)
	}

	// deal: no unique constraint keyed off organization_id/partner_org_id
	// alone, safe to move directly.
	if _, err := tx.ExecContext(ctx, `UPDATE deal SET organization_id=$2::uuid WHERE organization_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink deal.organization_id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deal SET partner_org_id=$2::uuid WHERE partner_org_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink deal.partner_org_id: %w", err)
	}

	// relationship / employment + partner-kind edges (organization_id and
	// counterparty_org_id sides): no unique constraint keyed off either
	// column alone, safe to move directly.
	if _, err := tx.ExecContext(ctx, `UPDATE relationship SET organization_id=$2::uuid WHERE organization_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink relationship.organization_id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE relationship SET counterparty_org_id=$2::uuid WHERE counterparty_org_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink relationship.counterparty_org_id: %w", err)
	}

	// activity_link: no soft-delete column on this table — delete the exact
	// duplicate rather than moving it, when the activity is already linked
	// to the target (uq_activity_link).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM activity_link
		WHERE organization_id=$1::uuid AND activity_id IN (
			SELECT activity_id FROM activity_link WHERE organization_id=$2::uuid)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink activity_link dedupe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_link SET organization_id=$2::uuid WHERE organization_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink activity_link: %w", err)
	}

	// partner: 1:1 UNIQUE(organization_id) — if the target already has a
	// live partner row, archive the loser's instead of violating the
	// constraint; otherwise move it onto the target.
	if _, err := tx.ExecContext(ctx, `
		UPDATE partner SET archived_at=now()
		WHERE organization_id=$1::uuid AND archived_at IS NULL
		  AND EXISTS (SELECT 1 FROM partner WHERE organization_id=$2::uuid AND archived_at IS NULL)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink partner collapse: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE partner SET organization_id=$2::uuid WHERE organization_id=$1::uuid AND archived_at IS NULL`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink partner: %w", err)
	}
	return nil
}
