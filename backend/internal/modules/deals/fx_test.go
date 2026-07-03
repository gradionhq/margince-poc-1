//go:build integration

package deals

import (
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

func openFXTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set — run via `make test-integration`")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedFXWorkspace creates a fresh, uniquely-id'd EUR-base workspace and returns its id — a
// distinct workspace per test avoids fx_rate row collisions/pollution across tests sharing
// the same (workspace_id, from_currency, to_currency, rate_date) space (uq_fx_rate).
func seedFXWorkspace(t *testing.T, db *sql.DB, tag string) string {
	t.Helper()
	tag = tag + "-" + time.Now().Format("20060102150405.000000000")

	var workspaceID string
	if err := db.QueryRow(`INSERT INTO workspace (id, name, slug, base_currency)
		VALUES (uuidv7(),'t13-fx-ws',$1,'EUR') RETURNING id`, "t13-fx-ws-"+tag).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return workspaceID
}

func TestAsOfFXRate_ExactDateMatch(t *testing.T) {
	db := openFXTestDB(t)
	workspaceID := seedFXWorkspace(t, db, "exact")
	asOf := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1,'USD','EUR',0.92,$2)`, workspaceID, asOf); err != nil {
		t.Fatalf("seed rate: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	rate, err := AsOfFXRate(t.Context(), tx, workspaceID, "USD", "EUR", asOf)
	if err != nil {
		t.Fatalf("AsOfFXRate: %v", err)
	}
	if rate != 0.92 {
		t.Fatalf("rate = %v, want 0.92", rate)
	}
}

func TestAsOfFXRate_MostRecentBeforeAsOf(t *testing.T) {
	db := openFXTestDB(t)
	workspaceID := seedFXWorkspace(t, db, "recent")
	earlier := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC) // after asOf — must be ignored
	asOf := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO fx_rate (workspace_id, from_currency, to_currency, rate, rate_date)
		VALUES ($1,'USD','EUR',0.90,$2), ($1,'USD','EUR',0.99,$3)`,
		workspaceID, earlier, later); err != nil {
		t.Fatalf("seed rates: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	rate, err := AsOfFXRate(t.Context(), tx, workspaceID, "USD", "EUR", asOf)
	if err != nil {
		t.Fatalf("AsOfFXRate: %v", err)
	}
	if rate != 0.90 {
		t.Fatalf("rate = %v, want 0.90 (the most recent rate <= as_of, never the later 0.99)", rate)
	}
}

func TestAsOfFXRate_NoRowReturnsStructuredError(t *testing.T) {
	db := openFXTestDB(t)
	workspaceID := seedFXWorkspace(t, db, "missing")
	asOf := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = AsOfFXRate(t.Context(), tx, workspaceID, "GBP", "EUR", asOf)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var fxErr *FXRateUnavailableError
	if !errors.As(err, &fxErr) {
		t.Fatalf("expected *FXRateUnavailableError, got %T: %v", err, err)
	}
	if fxErr.Currency != "GBP" || !fxErr.AsOf.Equal(asOf) {
		t.Fatalf("fxErr = %+v, want Currency=GBP AsOf=%v", fxErr, asOf)
	}
	if !errors.Is(err, errs.ErrFXRateUnavailable) {
		t.Fatal("error must wrap errs.ErrFXRateUnavailable (422 mapping)")
	}
}
