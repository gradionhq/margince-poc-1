package crmaudit

import (
	"context"
	"database/sql"
	"fmt"

	database "github.com/gradionhq/margince/backend/internal/platform/database"
)

// SmellRow is the manual-entry-smell breakdown for one channel.
type SmellRow struct {
	Channel   string
	Total     int
	Manual    int
	ManualPct float64
}

// ManualEntrySmell computes the per-channel manual-entry share for a workspace:
// fraction of activities AND people created with captured_by = 'human:%'.
// Agent- and connector-captured rows count as captured, not manual.
// Channel = source_system (NULL -> "direct").
func ManualEntrySmell(ctx context.Context, db *sql.DB, workspaceID string) ([]SmellRow, error) {
	var out []SmellRow
	err := database.WithWorkspaceTx(ctx, db, workspaceID, func(tx *sql.Tx) error {
		rs, err := tx.QueryContext(ctx, `
			WITH provrows AS (
			  SELECT source_system, captured_by FROM activity
			    WHERE workspace_id = $1::uuid AND archived_at IS NULL
			  UNION ALL
			  SELECT source_system, captured_by FROM person
			    WHERE workspace_id = $1::uuid AND archived_at IS NULL
			)
			SELECT coalesce(source_system,'direct') AS channel,
			       count(*) AS total,
			       count(*) FILTER (WHERE captured_by LIKE 'human:%') AS manual
			FROM provrows
			GROUP BY coalesce(source_system,'direct')
			ORDER BY channel`, workspaceID)
		if err != nil {
			return err
		}
		defer func() { _ = rs.Close() }()
		for rs.Next() {
			var r SmellRow
			if err := rs.Scan(&r.Channel, &r.Total, &r.Manual); err != nil {
				return err
			}
			if r.Total > 0 {
				r.ManualPct = float64(r.Manual) / float64(r.Total)
			}
			out = append(out, r)
		}
		return rs.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("crmaudit smell: %w", err)
	}
	return out, nil
}
