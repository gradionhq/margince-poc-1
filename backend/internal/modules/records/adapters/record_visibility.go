package adapters

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

// RecordVisible reports whether principal can see the bound record
// (entityType/entityID) per perms's row_scope for that object — Constraint
// 6: attachment visibility is exactly the bound record's visibility, not a
// separate ACL. entityType must be one of domain.EntityType{Person,
// Organization,Deal,Lead,Activity}.
//
// Returns (false, nil) for object entries absent from perms (caller cannot
// read the bound record's own object at all), so the attachment handler can
// treat it as a disclosed-locked row without a 404.
func RecordVisible(ctx context.Context, db *sql.DB, workspaceID, entityType, entityID string, principal crmctx.Principal, perms session.RolePermissions) (bool, error) {
	entry, ok := perms[entityType]
	if !ok {
		return false, nil
	}
	rule, ok := entry.Actions["read"]
	if !ok {
		return false, nil
	}
	if rule.RowScope == rowScopeAll {
		return true, nil
	}
	// activity has no per-row ownership model anywhere in this codebase, and
	// record_grant's own DB CHECK excludes 'activity' — treat any row_scope
	// that passed the object-level gate above as visible.
	if entityType == domain.EntityTypeActivity {
		return true, nil
	}
	// query is one of a fixed set of literal (non-interpolated) SELECTs, one
	// per owned entity type — mirrors the discipline in activities/adapters'
	// insertLink switch, so no query text is built from runtime data.
	var query string
	switch entityType {
	case domain.EntityTypePerson:
		query = `SELECT owner_id FROM person WHERE id=$1 AND workspace_id=$2`
	case domain.EntityTypeOrganization:
		query = `SELECT owner_id FROM organization WHERE id=$1 AND workspace_id=$2`
	case domain.EntityTypeDeal:
		query = `SELECT owner_id FROM deal WHERE id=$1 AND workspace_id=$2`
	case domain.EntityTypeLead:
		query = `SELECT owner_id FROM lead WHERE id=$1 AND workspace_id=$2`
	default:
		return false, nil
	}
	var ownerID sql.NullString
	err := db.QueryRowContext(ctx, query, entityID, workspaceID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if ownerID.Valid && ownerID.String == principal.UserID {
		return true, nil
	}
	// record_grant widens access — same predicate as buildOrgListWhere's
	// EXISTS sub-select (organizations/adapters/store_org_list.go), reused
	// verbatim as a point-lookup instead of a list WHERE-clause fragment.
	var grantExists bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM record_grant
		WHERE workspace_id=$1 AND record_type=$2 AND record_id=$3
		  AND subject_type='user' AND subject_id=$4
		  AND (expires_at IS NULL OR expires_at > now()))`,
		workspaceID, entityType, entityID, principal.UserID).Scan(&grantExists)
	return grantExists, err
}
