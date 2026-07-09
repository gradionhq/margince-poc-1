package adapters

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

// OpenPipelineRollup reads the organization_open_pipeline_rollup view for one organization,
// scoped to the workspace as a defense-in-depth join filter. It returns nil, 0, nil when the
// organization has no open deals at all.
func (s *RollupStore) OpenPipelineRollup(ctx context.Context, orgID, workspaceID string) (openPipelineMinorBase *int64, openDealCount int, err error) {
	var minor sql.NullInt64
	err = s.db.QueryRowContext(ctx, `
		SELECT r.open_pipeline_minor_base, r.open_deal_count
		FROM organization_open_pipeline_rollup r
		JOIN organization o ON o.id = r.organization_id
		WHERE r.organization_id = $1::uuid AND o.workspace_id = $2::uuid`,
		orgID, workspaceID).Scan(&minor, &openDealCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if minor.Valid {
		v := minor.Int64
		openPipelineMinorBase = &v
	}
	return openPipelineMinorBase, openDealCount, nil
}

// ComputedFieldsVisible reports whether the principal's role permissions grant computed_field:read
// for the workspace.
func (s *RollupStore) ComputedFieldsVisible(ctx context.Context, workspaceID string, principal crmctx.Principal) (bool, error) {
	perms, err := auth.LoadRolePermissions(ctx, s.db, workspaceID, principal.UserID)
	if err != nil {
		return false, err
	}
	return session.AuthorizePerms(perms, auth.ObjComputedField, auth.ActionRead) == nil, nil
}
