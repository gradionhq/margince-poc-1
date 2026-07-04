// Package crmcore — PersonStore.Merge (PO-AC-17, PO-AC-M1 through PO-AC-M5).
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

// ErrSelfMerge is returned when target_id == id (PO-AC-M3). Maps to 422.
var ErrSelfMerge = errors.New("cannot merge a person into itself")

// ErrAlreadyMerged is returned when the id being merged (the loser candidate)
// already has merged_into_id set — following the chain to the final survivor
// (PO-AC-M4, UAT step 3). Maps to 422.
type ErrAlreadyMerged struct{ SurvivorID string }

func (e *ErrAlreadyMerged) Error() string {
	return fmt.Sprintf("already merged into %s", e.SurvivorID)
}

// ErrMergeTargetInvalid is returned when the merge TARGET is archived or is
// itself already merged elsewhere — following the chain to the actual
// survivor (PO-AC-M4). Maps to 422.
type ErrMergeTargetInvalid struct{ SurvivorID string }

func (e *ErrMergeTargetInvalid) Error() string {
	return fmt.Sprintf("merge target is invalid; actual survivor is %s", e.SurvivorID)
}

// followMergeChain walks merged_into_id until it finds a person with no
// further merge pointer, returning that final id. Used so a 422 always points
// at the true current survivor, never a stale intermediate hop.
func followMergeChain(ctx context.Context, tx *sql.Tx, id, workspaceID string) (string, error) {
	current := id
	for {
		var next sql.NullString
		if err := tx.QueryRowContext(ctx,
			`SELECT merged_into_id::text FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			current, workspaceID).Scan(&next); err != nil {
			return "", err
		}
		if !next.Valid {
			return current, nil
		}
		current = next.String
	}
}

// mergeLoserState holds state read during validateMergePair.
type mergeLoserState struct {
	version   int64
	beforeRaw []byte
}

// validateMergePair reads both loser and target rows inside tx, returning the
// loser's version + before-snapshot or a typed 422/404 error if the pair is
// not eligible for merge (already-merged loser, invalid target).
//
//nolint:dupl // parallel per-entity merge validation: mirrored by validateOrgMergePair (store_merge_org.go) for organization; the SQL table names and error wiring differ by entity, a generic version would read worse than the explicit form
func validateMergePair(ctx context.Context, tx *sql.Tx, loserID, targetID, workspaceID string) (mergeLoserState, error) {
	var state mergeLoserState
	var loserMergedInto sql.NullString
	var loserArchived sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT version, merged_into_id::text, archived_at, row_to_json(person.*)
		FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		loserID, workspaceID).Scan(&state.version, &loserMergedInto, &loserArchived, &state.beforeRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, errs.ErrNotFound
		}
		return state, err
	}
	if loserMergedInto.Valid {
		survivor, err := followMergeChain(ctx, tx, loserID, workspaceID)
		if err != nil {
			return state, err
		}
		return state, &ErrAlreadyMerged{SurvivorID: survivor}
	}

	var targetMergedInto sql.NullString
	var targetArchived sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`SELECT merged_into_id::text, archived_at FROM person WHERE id=$1::uuid AND workspace_id=$2::uuid`,
		targetID, workspaceID).Scan(&targetMergedInto, &targetArchived); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return state, errs.ErrNotFound
		}
		return state, err
	}
	if targetMergedInto.Valid || targetArchived.Valid {
		survivor, err := followMergeChain(ctx, tx, targetID, workspaceID)
		if err != nil {
			return state, err
		}
		return state, &ErrMergeTargetInvalid{SurvivorID: survivor}
	}
	return state, nil
}

// Merge relinks loserID's emails, phones, employment/deal-stakeholder
// relationships, and activity links onto targetID, archives loserID with
// merged_into_id=targetID, and writes one audit_log row (action "merge",
// before=full pre-merge loser snapshot for reversibility) plus one
// person.merged event_outbox row — all in a single workspace-scoped tx.
//
// FK enumeration (grep "REFERENCES person(id)" backend/migrations/*.sql):
// person_email, person_phone, relationship (employment + deal_stakeholder
// kinds), activity_link are actively relinked because they describe the
// person's *current* state. consent, consent_event, and lead.promoted_person_id
// describe historical facts about a specific person id at capture time and are
// deliberately left pointing at the archived loser — the row still exists
// (soft-archived, never deleted), so this is not an orphaned FK, just an
// unmoved historical record. TestPersonMergeFKWalkExhaustive proves this list
// is exhaustive against the live schema, not just this comment's memory of it.
func (s *PersonStore) Merge(ctx context.Context, loserID, targetID, workspaceID string) (Person, error) {
	if loserID == targetID {
		return Person{}, ErrSelfMerge
	}
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		state, err := validateMergePair(ctx, tx, loserID, targetID, workspaceID)
		if err != nil {
			return err
		}
		if err := relinkPersonFKs(ctx, tx, loserID, targetID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE person SET merged_into_id=$3::uuid, archived_at=now()
			WHERE id=$1::uuid AND workspace_id=$2::uuid AND version=$4 AND merged_into_id IS NULL AND archived_at IS NULL`,
			loserID, workspaceID, targetID, state.version)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errs.ErrVersionSkew
		}
		e := crmaudit.EntryFromPrincipal(ctx, "merge", entityTypePerson, &loserID, json.RawMessage(state.beforeRaw), map[string]any{fieldMergedIntoID: targetID})
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("person merge audit: %w", err)
		}
		payload := marshalJSON(map[string]any{"person_id": loserID, "merged_into_id": targetID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,'person.merged',$2::uuid,$3)`,
			workspaceID, loserID, payload); err != nil {
			return fmt.Errorf("person merge event: %w", err)
		}
		return nil
	})
	if err != nil {
		return Person{}, err
	}
	return s.Get(ctx, targetID, workspaceID)
}

// relinkPersonFKs moves loserID's email/phone/relationship/activity_link rows
// onto targetID, demoting (never deleting) any conflicting is_primary/
// current-primary row on the SURVIVOR side per PO-AC-M1, and collapsing
// duplicate deal-stakeholder rows per PO-AC-M2 instead of violating
// uq_rel_deal_person_role.
func relinkPersonFKs(ctx context.Context, tx *sql.Tx, loserID, targetID string) error {
	// person_email: drop any loser email that collides with a live target
	// email (uq_person_email is workspace-wide on lower(email)); move the
	// rest. If the moved row is_primary and the target already has a primary
	// email, demote it (target's original primary wins, PO-AC-M1).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM person_email
		WHERE person_id=$1::uuid AND archived_at IS NULL AND lower(email) IN (
			SELECT lower(email) FROM person_email WHERE person_id=$2::uuid AND archived_at IS NULL)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink person_email dedupe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE person_email SET person_id=$2::uuid,
		  is_primary = is_primary AND NOT EXISTS (
		    SELECT 1 FROM person_email WHERE person_id=$2::uuid AND is_primary AND archived_at IS NULL)
		WHERE person_id=$1::uuid AND archived_at IS NULL`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink person_email: %w", err)
	}

	// person_phone: demote the loser's primary-per-type row(s) first if the
	// target already holds a live primary of that phone_type
	// (uq_person_phone_primary), then move everything.
	if _, err := tx.ExecContext(ctx, `
		UPDATE person_phone SET is_primary=false
		WHERE person_id=$1::uuid AND is_primary=true AND archived_at IS NULL AND phone_type IN (
			SELECT phone_type FROM person_phone WHERE person_id=$2::uuid AND is_primary=true AND archived_at IS NULL)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink person_phone demote: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE person_phone SET person_id=$2::uuid WHERE person_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink person_phone: %w", err)
	}

	// relationship / employment: demote loser's current-primary employer row
	// if target already has one live (uq_rel_current_primary_employer), then move.
	if _, err := tx.ExecContext(ctx, `
		UPDATE relationship SET is_primary=false
		WHERE person_id=$1::uuid AND kind='employment' AND is_primary=true AND ended_at IS NULL AND archived_at IS NULL
		  AND EXISTS (SELECT 1 FROM relationship WHERE person_id=$2::uuid AND kind='employment' AND is_primary=true AND ended_at IS NULL AND archived_at IS NULL)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink employment demote: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE relationship SET person_id=$2::uuid WHERE person_id=$1::uuid AND kind='employment'`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink employment: %w", err)
	}

	// relationship / deal_stakeholder: archive the loser's duplicate row when
	// the target already has a live (deal_id, role) row (uq_rel_deal_person_role
	// dedupe-collapse, PO-AC-M2); move everything else.
	if _, err := tx.ExecContext(ctx, `
		UPDATE relationship SET archived_at=now()
		WHERE person_id=$1::uuid AND kind='deal_stakeholder' AND archived_at IS NULL
		  AND (deal_id, role) IN (
		    SELECT deal_id, role FROM relationship WHERE person_id=$2::uuid AND kind='deal_stakeholder' AND archived_at IS NULL)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink deal_stakeholder collapse: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE relationship SET person_id=$2::uuid WHERE person_id=$1::uuid AND kind='deal_stakeholder' AND archived_at IS NULL`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink deal_stakeholder: %w", err)
	}

	// activity_link: no soft-delete column on this table — delete the exact
	// duplicate rather than moving it, when the activity is already linked to
	// the target (uq_activity_link).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM activity_link
		WHERE person_id=$1::uuid AND activity_id IN (
			SELECT activity_id FROM activity_link WHERE person_id=$2::uuid)`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink activity_link dedupe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_link SET person_id=$2::uuid WHERE person_id=$1::uuid`,
		loserID, targetID); err != nil {
		return fmt.Errorf("relink activity_link: %w", err)
	}
	return nil
}
