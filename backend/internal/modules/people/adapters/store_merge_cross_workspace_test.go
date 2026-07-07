//go:build integration

package adapters_test

import (
	"errors"
	"testing"

	adapters "github.com/gradionhq/margince/backend/internal/modules/people/adapters"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// PO-AC-M6: cross-workspace merge is impossible by construction — RLS (not
// app code) blocks it. A merge attempt where the loser/target ids live in a
// DIFFERENT workspace than the one passed to Merge must behave exactly like
// "not found" — the row is invisible under RLS, never a leak or a different
// error class.
func TestPersonMergeCrossWorkspaceBlockedByRLS(t *testing.T) {
	db := openTestDB(t)
	wsA, wsB := ids.New(), ids.New()
	seedWorkspace(t, db, wsA)
	seedWorkspace(t, db, wsB)
	store := adapters.NewPersonStore(db)
	inA := mkPerson(mergeTestCtx(wsA), t, store, wsA, "InA")
	inB := mkPerson(mergeTestCtx(wsB), t, store, wsB, "InB")

	// Attempt the merge scoped to wsB — inA is invisible under RLS.
	_, err := store.Merge(mergeTestCtx(wsB), inA.ID, inB.ID, wsB)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("cross-workspace person merge: want ErrNotFound (RLS-invisible), got %v", err)
	}
}
