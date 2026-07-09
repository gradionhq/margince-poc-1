package adapters

import (
	"context"

	"github.com/gradionhq/margince/backend/internal/platform/customfields"
)

// ActiveCustomFieldNames returns the workspace's active custom-column wire keys
// (e.g. ["cf_org_note"]) for the organization object — the thin passthrough the
// transport handler merges into its sort/filter vocabulary without importing
// customfields directly.
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

// The comma-prefixed quoted SELECT suffix ($N-placeholder INSERT column list,
// SET-clause fragments) is mechanically identical to person's — those helpers
// live once, in customfields.SelectSuffix/InsertColumns/UpdateSetClauses, and
// are called directly at each org call site below instead of re-implemented
// here.
