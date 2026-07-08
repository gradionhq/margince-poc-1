//go:build integration

// Package records_test is a test-only package proving the formula-field boundary (RD-AC-6/RD-AC-7,
// RD-AC-N-1, docs/subsystems/records-depth.md): a formula field is a database-GENERATED column,
// never a runtime-authored one. No domain/ports/adapters/app/transport code lives here — this
// module exists purely to house the schema, negative-scope, and bound proofs against the
// deal.amount_minor_base column + organization_open_pipeline_rollup view added by
// backend/migrations/000075_formula_field_boundary.up.sql.
package records_test

import (
	"strings"
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// TestDealAmountMinorBaseIsGenerated is RD-AC-6's schema test: it introspects the live migrated
// schema (information_schema.columns) and asserts deal.amount_minor_base is a genuine
// database-computed GENERATED column -- is_generated = 'ALWAYS' with a non-empty
// generation_expression referencing its same-row inputs -- never an app-side interpreted
// expression. Structural proof only; behavior is covered by TestDealAmountMinorBase_Values in
// pipeline_rollup_bound_test.go.
func TestDealAmountMinorBaseIsGenerated(t *testing.T) {
	db := pgtest.OpenTestDB(t)

	var isGenerated, generationExpr string
	err := db.QueryRow(`
		SELECT is_generated, coalesce(generation_expression, '')
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'deal' AND column_name = 'amount_minor_base'
	`).Scan(&isGenerated, &generationExpr)
	if err != nil {
		t.Fatalf("introspect deal.amount_minor_base: %v (migration 000075 not applied?)", err)
	}

	if isGenerated != "ALWAYS" {
		t.Errorf("deal.amount_minor_base: is_generated = %q, want %q (RD-AC-6: must be a real DB "+
			"GENERATED column, not an app-side interpreted expression)", isGenerated, "ALWAYS")
	}
	if generationExpr == "" {
		t.Fatal("deal.amount_minor_base: generation_expression is empty -- expected a real stored expression")
	}
	for _, want := range []string{"amount_minor", "fx_rate_to_base"} {
		if !strings.Contains(generationExpr, want) {
			t.Errorf("deal.amount_minor_base generation_expression = %q, want it to reference %q "+
				"(proves this computes from the deal's own row, not a placeholder)", generationExpr, want)
		}
	}
}

// TestOrgOpenPipelineRollupIsARealSQLView proves the cross-record aggregate is served as a
// genuine SQL view (information_schema.views), reading the deal table directly -- reinforcing
// RD-AC-N-1's bound: never a runtime interpreter, a real database object whose definition is
// introspectable.
func TestOrgOpenPipelineRollupIsARealSQLView(t *testing.T) {
	db := pgtest.OpenTestDB(t)

	var viewDef string
	err := db.QueryRow(`
		SELECT view_definition FROM information_schema.views
		WHERE table_schema = 'public' AND table_name = 'organization_open_pipeline_rollup'
	`).Scan(&viewDef)
	if err != nil {
		t.Fatalf("introspect organization_open_pipeline_rollup: %v (migration 000075 not applied?)", err)
	}
	if !strings.Contains(strings.ToLower(viewDef), "deal") {
		t.Errorf("organization_open_pipeline_rollup view_definition = %q, want it to read the deal table", viewDef)
	}
}
