package adapters

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gradionhq/margince/backend/internal/modules/organizations/domain"
	"github.com/gradionhq/margince/backend/internal/platform/customfields"
	"github.com/lib/pq"
)

func (s *OrgStore) ActiveCustomFieldNames(ctx context.Context, workspaceID string) ([]string, error) {
	cols, err := customfields.ActiveColumns(ctx, s.db, workspaceID, "organization")
	if err != nil {
		return nil, err
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.ColumnName
	}
	return names, nil
}

func attachOrgCustomFields(ctx context.Context, db *sql.DB, workspaceID string, o *domain.Organization) error {
	cols, err := customfields.ActiveColumns(ctx, db, workspaceID, "organization")
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		o.CustomFields = nil
		return nil
	}
	return loadOrgCustomFields(ctx, db, workspaceID, o, cols)
}

func loadOrgCustomFields(ctx context.Context, db *sql.DB, workspaceID string, o *domain.Organization, cols []customfields.Column) error {
	if len(cols) == 0 {
		o.CustomFields = nil
		return nil
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = pq.QuoteIdentifier(c.ColumnName)
	}
	query := "SELECT " + strings.Join(parts, ", ") + " FROM organization WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL"
	row := db.QueryRowContext(ctx, query, o.ID, workspaceID)
	dests := customfields.ScanDests(cols)
	if err := row.Scan(dests...); err != nil {
		return err
	}
	o.CustomFields = customfields.ExtractValues(cols, dests)
	return nil
}
