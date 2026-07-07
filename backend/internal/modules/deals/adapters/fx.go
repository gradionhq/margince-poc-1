// Package adapters: FX as-of lookup for the open-deal roll-up (DEAL-FORM-2, DM-FX-5).
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// FXRateUnavailableError signals that no stored fx_rate row satisfies the as-of lookup for
// the requested currency pair.
type FXRateUnavailableError struct {
	Currency string
	AsOf     time.Time
}

func (e *FXRateUnavailableError) Error() string {
	return fmt.Sprintf("no stored fx_rate for %s as of %s", e.Currency, e.AsOf.Format("2006-01-02"))
}

func (e *FXRateUnavailableError) Unwrap() error { return errs.ErrFXRateUnavailable }

// AsOfFXRate returns the most recent fx_rate.rate for fromCurrency->toCurrency with
// rate_date <= asOf.
func AsOfFXRate(ctx context.Context, tx *sql.Tx, workspaceID, fromCurrency, toCurrency string, asOf time.Time) (float64, error) {
	var rate float64
	err := tx.QueryRowContext(ctx, `
		SELECT rate
		FROM fx_rate
		WHERE workspace_id=$1::uuid
		  AND from_currency=$2
		  AND to_currency=$3
		  AND rate_date <= $4::date
		ORDER BY rate_date DESC
		LIMIT 1`,
		workspaceID, fromCurrency, toCurrency, asOf.Format("2006-01-02")).Scan(&rate)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, &FXRateUnavailableError{Currency: fromCurrency, AsOf: asOf}
	}
	if err != nil {
		return 0, err
	}
	return rate, nil
}
