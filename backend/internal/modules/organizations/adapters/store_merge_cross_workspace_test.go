//go:build integration

// store_merge_cross_workspace_test.go — org half of
// modules/directory/store_merge_cross_workspace_test.go, ported to
// package adapters_test. Person half is in modules/people/adapters.
package adapters_test

import (
	"errors"
	"testing"

	orgAdapters "github.com/gradionhq/margince/backend/internal/modules/organizations/adapters"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

// PO-AC-M6: cross-workspace org merge is impossible by construction — RLS (not
// app code) blocks it. A merge attempt where the loser/target IDs live in a
// DIFFERENT workspace than the one passed to Merge must behave exactly like
// "not found" — the row is invisible under RLS, never a leak or a different
// error class.
func TestOrgMergeCrossWorkspaceBlockedByRLS(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsA, wsB := ids.New(), ids.New()
	pgtest.SeedWorkspace(t, db, wsA)
	pgtest.SeedWorkspace(t, db, wsB)
	orgStore := orgAdapters.NewOrgStore(db)
	inA := mkOrg(mergeTestCtx(wsA), t, orgStore, wsA, "InA Co")
	inB := mkOrg(mergeTestCtx(wsB), t, orgStore, wsB, "InB Co")

	_, err := orgStore.Merge(mergeTestCtx(wsB), inA.ID, inB.ID, wsB)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("cross-workspace org merge: want ErrNotFound (RLS-invisible), got %v", err)
	}
}
