// Package adapters — PersonStore.Restore, split out of store.go to keep it
// under the T1 500-LOC file cap (architecture/18 §3.2).
package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/people/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/workspacetx"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// Restore re-activates an archived person, writing one person.restored outbox
// event and one audit_log row in the same workspace-scoped tx. The record must
// already be archived and must not be a merge target.
func (s *PersonStore) Restore(ctx context.Context, id, workspaceID string) (domain.Person, error) {
	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
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
		// Inverse of Archive's cascade (store_person_archive.go): restoring a
		// person makes its per-person child rows live again, so they are properly
		// dedupe-checked against other live people going forward (T23 UAT
		// follow-up).
		if _, err := tx.ExecContext(ctx,
			`UPDATE person_email SET archived_at=NULL
			 WHERE person_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NOT NULL`,
			id, workspaceID); err != nil {
			return fmt.Errorf("person restore cascade person_email: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE person_phone SET archived_at=NULL
			 WHERE person_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NOT NULL`,
			id, workspaceID); err != nil {
			return fmt.Errorf("person restore cascade person_phone: %w", err)
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
		return domain.Person{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}

// findDuplicateEmail returns ErrDuplicateEmail when the person being restored
// (id, currently still archived — and, per Archive's cascade, so are its own
// person_email rows at this point in the tx, before the inverse cascade below
// runs) has an email that collides with another live person's email in the
// same workspace (used by Restore to re-check the invariant Create enforces
// at insert time). pe1 (the restoring person's own rows) is intentionally not
// filtered on archived_at — it is expected to still be archived here; only
// pe2 (the other candidate person) must be live for a collision to count.
func (s *PersonStore) findDuplicateEmail(ctx context.Context, tx *sql.Tx, workspaceID, personID string) error {
	var existingID string
	if err := tx.QueryRowContext(ctx, `
		SELECT pe2.person_id
		FROM person_email pe1
		JOIN person_email pe2 ON lower(pe2.email) = lower(pe1.email)
		  AND pe2.workspace_id = pe1.workspace_id
		  AND pe2.person_id <> pe1.person_id
		  AND pe2.archived_at IS NULL
		WHERE pe1.person_id=$1::uuid AND pe1.workspace_id=$2::uuid
		LIMIT 1`,
		personID, workspaceID).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return &ErrDuplicateEmail{ExistingID: existingID, Field: "email"}
}
