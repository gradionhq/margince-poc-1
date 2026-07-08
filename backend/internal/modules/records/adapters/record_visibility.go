package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/ports/session"
)

// entityOwnerTable maps each owned entity type to its DB table name — a fixed
// literal set, never interpolated from caller input (mirrors the discipline in
// activities/adapters' insertLink switch). Only person/organization/deal/lead
// have an owner_id column; activity is handled separately below.
var entityOwnerTable = map[string]string{
	domain.EntityTypePerson:       "person",
	domain.EntityTypeOrganization: "organization",
	domain.EntityTypeDeal:         "deal",
	domain.EntityTypeLead:         "lead",
}

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
	if rule.RowScope == "all" {
		return true, nil
	}
	// activity has no per-row ownership model anywhere in this codebase, and
	// record_grant's own DB CHECK excludes 'activity' — treat any row_scope
	// that passed the object-level gate above as visible.
	if entityType == domain.EntityTypeActivity {
		return true, nil
	}
	table, ok := entityOwnerTable[entityType]
	if !ok {
		return false, nil
	}
	var ownerID sql.NullString
	err := db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT owner_id FROM %s WHERE id=$1 AND workspace_id=$2`, table),
		entityID, workspaceID).Scan(&ownerID)
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
