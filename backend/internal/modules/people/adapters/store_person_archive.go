// Package adapters — PersonStore.Archive, split out of store.go to keep it
// under the T1 500-LOC file cap (architecture/18 §3.2).
package adapters

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/people/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/platform/workspacetx"
)

// Archive soft-deletes a person and returns the archived entity.
func (s *PersonStore) Archive(ctx context.Context, id, workspaceID string) (domain.Person, error) {
	err := workspacetx.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE person SET archived_at=now() WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
			id, workspaceID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			// Cascade archived_at onto per-person child rows with their own
			// archived_at column, so live-scoped uniqueness checks (e.g.
			// PO-AC-16 email dedupe) stop counting the archived person's
			// rows as live (T23 UAT follow-up).
			if _, err := tx.ExecContext(ctx,
				`UPDATE person_email SET archived_at=now()
				 WHERE person_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
				id, workspaceID); err != nil {
				return fmt.Errorf("person archive cascade person_email: %w", err)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE person_phone SET archived_at=now()
				 WHERE person_id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`,
				id, workspaceID); err != nil {
				return fmt.Errorf("person archive cascade person_phone: %w", err)
			}
			ea := crmaudit.EntryFromPrincipal(ctx, "archive", entityTypePerson, &id, nil, nil)
			ea.WorkspaceID = workspaceID
			if _, err := crmaudit.WriteTx(ctx, tx, ea); err != nil {
				return fmt.Errorf("person archive audit: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Person{}, err
	}
	return s.GetAny(ctx, id, workspaceID)
}
