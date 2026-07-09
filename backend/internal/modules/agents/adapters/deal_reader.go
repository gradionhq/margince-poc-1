package adapters

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
)

// SQLDealReader reads deal snapshots from the deals tables.
type SQLDealReader struct {
	db *sql.DB
}

// NewSQLDealReader returns a DealReader backed by the workspace database.
func NewSQLDealReader(db *sql.DB) *SQLDealReader {
	return &SQLDealReader{db: db}
}

var _ ports.DealReader = (*SQLDealReader)(nil)

// ListOpenDeals returns the workspace's live open deals with the values the
// close-date hygiene pass needs already resolved.
func (r *SQLDealReader) ListOpenDeals(ctx context.Context, workspaceID string, now time.Time) ([]ports.DealSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.id, d.workspace_id, d.pipeline_id, d.status,
		       d.expected_close_date, d.forecast_category, d.version,
		       s.position, s.win_probability, s.semantic,
		       d.last_activity_at, d.wait_until, d.created_at
		FROM deal d
		JOIN stage s ON s.id = d.stage_id AND s.pipeline_id = d.pipeline_id AND s.workspace_id = d.workspace_id
		WHERE d.workspace_id = $1::uuid AND d.status = 'open' AND d.archived_at IS NULL
		ORDER BY d.id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var snaps []ports.DealSnapshot
	for rows.Next() {
		var (
			snap                                  ports.DealSnapshot
			expectedCloseDate                     sql.NullTime
			forecastCategory                      sql.NullString
			stagePosition                         int
			winProbability                        int
			stageSemantic                         string
			lastActivityAt, waitUntil             sql.NullTime
			createdAt                             time.Time
		)
		if err := rows.Scan(&snap.DealID, &snap.WorkspaceID, &snap.PipelineID, &snap.Status,
			&expectedCloseDate, &forecastCategory, &snap.Version,
			&stagePosition, &winProbability, &stageSemantic,
			&lastActivityAt, &waitUntil, &createdAt); err != nil {
			return nil, err
		}
		if expectedCloseDate.Valid {
			snap.ExpectedCloseDate = &expectedCloseDate.Time
		}
		if forecastCategory.Valid {
			snap.ForecastCategory = &forecastCategory.String
		}
		snap.WinProbability = winProbability
		snap.RemainingOpenStages = 0
		if stageSemantic == "open" {
			var count int
			if err := r.db.QueryRowContext(ctx, `
				SELECT count(*)
				FROM stage
				WHERE workspace_id=$1::uuid AND pipeline_id=$2::uuid AND archived_at IS NULL
				  AND semantic='open' AND position >= $3`,
				workspaceID, snap.PipelineID, stagePosition).Scan(&count); err != nil {
				return nil, err
			}
			snap.RemainingOpenStages = count
		}
		deal := deals.Deal{
			Status:            snap.Status,
			ExpectedCloseDate: snap.ExpectedCloseDate,
			CreatedAt:         createdAt,
		}
		if lastActivityAt.Valid {
			deal.LastActivityAt = &lastActivityAt.Time
		}
		if waitUntil.Valid {
			w := waitUntil.Time
			deal.WaitUntil = &w
		}
		snap.IsStalled, _ = deals.IsStalled(deal, now)
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return snaps, nil
}

// PipelineWonVelocity returns the raw median days-per-stage over won deals in
// the pipeline and how many won deals informed it.
func (r *SQLDealReader) PipelineWonVelocity(ctx context.Context, workspaceID, pipelineID string) (int, int, error) {
	var wonDealCount int
	if err := r.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM deal
		WHERE workspace_id=$1::uuid AND pipeline_id=$2::uuid AND status='won' AND archived_at IS NULL`,
		workspaceID, pipelineID).Scan(&wonDealCount); err != nil {
		return 0, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT d.id, h.occurred_at
		FROM deal d
		JOIN deal_stage_history h ON h.deal_id = d.id AND h.workspace_id = d.workspace_id
		WHERE d.workspace_id = $1::uuid AND d.pipeline_id = $2::uuid AND d.status = 'won' AND d.archived_at IS NULL
		ORDER BY d.id, h.occurred_at`, workspaceID, pipelineID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()

	type history struct {
		dealID string
		at     time.Time
	}
	byDeal := map[string][]time.Time{}
	for rows.Next() {
		var dealID string
		var occurredAt time.Time
		if err := rows.Scan(&dealID, &occurredAt); err != nil {
			return 0, 0, err
		}
		byDeal[dealID] = append(byDeal[dealID], occurredAt)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	var samples []int
	for _, times := range byDeal {
		if len(times) < 2 {
			continue
		}
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		for i := 1; i < len(times); i++ {
			samples = append(samples, int(times[i].Sub(times[i-1]).Hours()/24))
		}
	}
	if len(samples) == 0 {
		return 0, wonDealCount, nil
	}
	sort.Ints(samples)
	median := samples[len(samples)/2]
	return median, wonDealCount, nil
}
