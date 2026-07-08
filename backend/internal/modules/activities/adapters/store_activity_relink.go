package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// Relink adds or moves one typed activity_link for the given entity_type.
// A repeated call for the same activity, entity_type, and entity_id is a no-op.
func (s *ActivityStore) Relink(ctx context.Context, activityID, workspaceID, entityType, entityID string) (domain.Activity, error) {
	if _, ok := entityLinkColumn[entityType]; !ok {
		return domain.Activity{}, errs.ErrInvalidLinkEntityType
	}

	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		if err := lockActivityForMutation(ctx, tx, activityID, workspaceID); err != nil {
			return err
		}

		existing, err := s.selectLinksByType(ctx, tx, activityID, entityType)
		if err != nil {
			return err
		}
		if len(existing) == 1 && existing[0].EntityID == entityID {
			return nil
		}

		if _, err := tx.ExecContext(ctx,
			`DELETE FROM activity_link WHERE activity_id=$1::uuid AND entity_type=$2`,
			activityID, entityType); err != nil {
			return fmt.Errorf("activity relink delete: %w", err)
		}
		if err := s.insertLink(ctx, tx, activityID, workspaceID, domain.ActivityLink{EntityType: entityType, EntityID: entityID}); err != nil {
			return err
		}
		e := crmaudit.EntryFromPrincipal(ctx, "activity_relink", entityTypeActivity, &activityID, nil,
			map[string]any{"entity_type": entityType, "entity_id": entityID})
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("activity relink audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Activity{}, err
	}
	return s.Get(ctx, activityID, workspaceID)
}

// lockActivityForMutation SELECT ... FOR UPDATE-locks a live activity row and
// returns ErrNotFound when the activity is missing or archived.
func lockActivityForMutation(ctx context.Context, tx *sql.Tx, activityID, workspaceID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM activity
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		FOR UPDATE`,
		activityID, workspaceID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.ErrNotFound
	}
	return err
}

// selectLinksByType reads the activity_link rows for one activity/entity_type pair.
func (s *ActivityStore) selectLinksByType(ctx context.Context, tx *sql.Tx, activityID, entityType string) ([]domain.ActivityLink, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, entity_type, coalesce(person_id, organization_id, deal_id)
		 FROM activity_link
		 WHERE activity_id=$1::uuid AND entity_type=$2`,
		activityID, entityType)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	links := []domain.ActivityLink{}
	for rows.Next() {
		var l domain.ActivityLink
		l.ActivityID = activityID
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}
