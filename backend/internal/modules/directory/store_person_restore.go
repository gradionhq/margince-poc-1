// Package crmcore — PersonStore.Restore, split out of store.go to keep it
// under the T1 500-LOC file cap (architecture/18 §3.2).
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

// Restore re-activates an archived person, writing one person.restored outbox
// event and one audit_log row in the same workspace-scoped tx. The record must
// already be archived and must not be a merge target.
func (s *PersonStore) Restore(ctx context.Context, id, workspaceID string) (Person, error) {
	err := withWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		var archivedAt sql.NullTime
		var mergedInto sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT archived_at, merged_into_id
			FROM person
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&archivedAt, &mergedInto); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.ErrNotFound
			}
			return err
		}
		if !archivedAt.Valid {
			return errs.ErrNotArchived
		}
		if mergedInto.Valid {
			return errs.ErrMergedRecord
		}
		if err := s.findDuplicateEmail(ctx, tx, workspaceID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE person SET archived_at=NULL
			 WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NOT NULL`,
			id, workspaceID); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{fieldPersonID: id})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "person.restored", id, payload); err != nil {
			return fmt.Errorf("person restore event: %w", err)
		}
		er := crmaudit.EntryFromPrincipal(ctx, "restore", entityTypePerson, &id, nil, nil)
		er.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, er); err != nil {
			return fmt.Errorf("person restore audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return Person{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// findDuplicateEmail returns ErrDuplicateEmail when id's live email(s) collide
// with another live person's email in the same workspace (used by Restore to
// re-check the invariant Create enforces at insert time).
func (s *PersonStore) findDuplicateEmail(ctx context.Context, tx *sql.Tx, workspaceID, personID string) error {
	var existingID string
	if err := tx.QueryRowContext(ctx, `
		SELECT pe2.person_id
		FROM person_email pe1
		JOIN person_email pe2 ON lower(pe2.email) = lower(pe1.email)
		  AND pe2.workspace_id = pe1.workspace_id
		  AND pe2.person_id <> pe1.person_id
		  AND pe2.archived_at IS NULL
		WHERE pe1.person_id=$1::uuid AND pe1.workspace_id=$2::uuid AND pe1.archived_at IS NULL
		LIMIT 1`,
		personID, workspaceID).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return &ErrDuplicateEmail{ExistingID: existingID, Field: "email"}
}
