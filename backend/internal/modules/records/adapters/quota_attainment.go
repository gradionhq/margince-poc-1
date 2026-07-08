package adapters

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// AttainmentDeal is one closed-won deal contributing to a quota's attainment.
type AttainmentDeal struct {
	DealID         string `json:"deal_id"`
	BaseValueMinor int64  `json:"base_value_minor"`
}

// Attainment is the computed attainment for one quota (RD-FORM-2).
type Attainment struct {
	QuotaID           string           `json:"quota_id"`
	ClosedWonMinor    int64            `json:"closed_won_minor"`
	TargetMinor       int64            `json:"target_minor"`
	Currency          string           `json:"currency"`
	AttainmentPct     float64          `json:"attainment_pct"`
	GapMinor          int64            `json:"gap_minor"`
	PacePct           float64          `json:"pace_pct"`
	Band              string           `json:"band"`
	AsOfDate          time.Time        `json:"as_of_date"`
	ContributingDeals []AttainmentDeal `json:"contributing_deals"`
}

// ErrAttainmentTargetZero is returned when a quota's target_minor is zero (checked before any query).
var ErrAttainmentTargetZero = errors.New("attainment_target_zero")

// pacePct returns what percentage of the quota period has elapsed as of today.
// Returns 0 before period_start, 100 at/after period_end.
func pacePct(periodStart, periodEnd, today time.Time) float64 {
	if today.Before(periodStart) {
		return 0
	}
	if !today.Before(periodEnd) {
		return 100
	}
	elapsed := today.Sub(periodStart).Seconds()
	total := periodEnd.Sub(periodStart).Seconds()
	return elapsed / total * 100
}

// attainmentBand maps an attainment percentage to a display band (RD-PARAM-4):
// met ≥100, accent 60–99.999…, behind <60.
func attainmentBand(pct float64) string {
	switch {
	case pct >= 100:
		return "met"
	case pct >= 60:
		return "accent"
	default:
		return "behind"
	}
}

// Attainment computes attainment_pct/gap_minor/pace_pct/band at read time from
// deal.status='won' rows in the quota's [period_start, period_end] window (RD-FORM-2).
// Never cached — always live. target_minor==0 is refused before any query.
func (s *QuotaStore) Attainment(ctx context.Context, id, workspaceID string) (Attainment, error) {
	q, err := s.Get(ctx, id, workspaceID)
	if err != nil {
		return Attainment{}, err
	}
	if q.TargetMinor == 0 {
		return Attainment{}, ErrAttainmentTargetZero
	}
	asOf := time.Now().UTC()
	out := Attainment{
		QuotaID:           id,
		TargetMinor:       q.TargetMinor,
		ContributingDeals: []AttainmentDeal{},
		AsOfDate:          asOf,
	}
	err = database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		baseCurrency, err := s.loadBaseCurrency(ctx, tx, workspaceID)
		if err != nil {
			return err
		}
		out.Currency = baseCurrency
		targetBase := q.TargetMinor
		if q.Currency != baseCurrency {
			rate, err := deals.AsOfFXRate(ctx, tx, workspaceID, q.Currency, baseCurrency, asOf)
			if err != nil {
				return err
			}
			targetBase = deals.ConvertToBase(q.TargetMinor, q.Currency, baseCurrency, rate)
		}
		out.TargetMinor = targetBase

		ownerIDs, err := s.scopeOwnerIDs(ctx, tx, workspaceID, q)
		if err != nil {
			return err
		}
		dealRows, err := s.contributingDeals(ctx, tx, workspaceID, ownerIDs, q.PeriodStart, q.PeriodEnd)
		if err != nil {
			return err
		}
		out.ContributingDeals = dealRows
		for _, d := range dealRows {
			out.ClosedWonMinor += d.BaseValueMinor
		}
		return nil
	})
	if err != nil {
		return Attainment{}, err
	}
	out.AttainmentPct = float64(out.ClosedWonMinor) / float64(out.TargetMinor) * 100
	out.GapMinor = out.ClosedWonMinor - out.TargetMinor
	out.PacePct = pacePct(q.PeriodStart, q.PeriodEnd, asOf)
	out.Band = attainmentBand(out.AttainmentPct)
	return out, nil
}

func (s *QuotaStore) loadBaseCurrency(ctx context.Context, tx *sql.Tx, workspaceID string) (string, error) {
	var baseCurrency string
	if err := tx.QueryRowContext(ctx, `
		SELECT base_currency
		FROM workspace
		WHERE id=$1::uuid`,
		workspaceID).Scan(&baseCurrency); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errs.ErrNotFound
		}
		return "", err
	}
	return baseCurrency, nil
}

// scopeOwnerIDs returns the owner ids whose closed-won deals count toward this quota.
// Owner-scoped → [ownerID]; team-scoped → all team_membership user_ids.
func (s *QuotaStore) scopeOwnerIDs(ctx context.Context, tx *sql.Tx, workspaceID string, q Quota) ([]string, error) {
	if q.OwnerID != nil {
		return []string{*q.OwnerID}, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id
		FROM team_membership
		WHERE team_id=$1::uuid AND workspace_id=$2::uuid`,
		*q.TeamID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		ids = append(ids, uid)
	}
	return ids, rows.Err()
}

// contributingDeals returns the closed-won deals in the quota's period, scoped to ownerIDs.
// Deals with NULL amount_minor_base contribute 0 and are omitted from the result.
func (s *QuotaStore) contributingDeals(ctx context.Context, tx *sql.Tx, workspaceID string, ownerIDs []string, periodStart, periodEnd time.Time) ([]AttainmentDeal, error) {
	if len(ownerIDs) == 0 {
		return []AttainmentDeal{}, nil
	}
	// Exclusive upper bound: period_end + 1 day computed in Go to avoid SQL type inference
	// ambiguity with the $N + INTERVAL form (PostgreSQL cannot infer the type of $N alone).
	periodEndExclusive := periodEnd.Add(24 * time.Hour)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, amount_minor_base
		FROM deal
		WHERE workspace_id=$1::uuid
		  AND status='won'
		  AND archived_at IS NULL
		  AND owner_id = ANY($2::uuid[])
		  AND closed_at >= $3
		  AND closed_at < $4`,
		workspaceID, pq.Array(ownerIDs), periodStart, periodEndExclusive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []AttainmentDeal{}
	for rows.Next() {
		var dealID string
		var baseVal sql.NullInt64
		if err := rows.Scan(&dealID, &baseVal); err != nil {
			return nil, err
		}
		if !baseVal.Valid {
			continue
		}
		out = append(out, AttainmentDeal{DealID: dealID, BaseValueMinor: baseVal.Int64})
	}
	return out, rows.Err()
}
