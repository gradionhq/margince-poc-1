//go:build integration

package deals

import (
	"context"
	"errors"
	"sync"
	"testing"

	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

const wsConcurrentReorder = "00000000-0000-0000-0000-000000000021"

// TestStageStore_Update_ConcurrentSwap_NoUniquenessViolationWindow proves
// DEAL-AC-12/UAT-6: two goroutines concurrently PATCH-swap two stages'
// positions in the same pipeline. Postgres's row-level locking on the
// uq_stage_position partial unique index means at most one of the two writes
// can win a genuine collision; the other must fail CLEANLY (a translated
// errs.ErrConflict, never a raw DB panic or a deadlock), and after both
// goroutines finish, the DB must never show two live stages sharing a
// position in this pipeline (the invariant the unique index itself
// guarantees, but which cleanly-translated errors must not defeat by
// leaving the transaction half-applied).
func TestStageStore_Update_ConcurrentSwap_NoUniquenessViolationWindow(t *testing.T) {
	db := openTestDB(t)
	setRLS(t, db, wsConcurrentReorder)
	seedWorkspace(t, db, wsConcurrentReorder)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsConcurrentReorder, Name: "Reorder " + uniq()})
	if err != nil {
		t.Fatal("create pipeline:", err)
	}
	sstore := NewStageStore(db)
	a, err := sstore.Create(ctx, Stage{WorkspaceID: wsConcurrentReorder, PipelineID: pl.ID, Name: "A", Position: 1, Semantic: "open"})
	if err != nil {
		t.Fatal("create stage A:", err)
	}
	b, err := sstore.Create(ctx, Stage{WorkspaceID: wsConcurrentReorder, PipelineID: pl.ID, Name: "B", Position: 2, Semantic: "open"})
	if err != nil {
		t.Fatal("create stage B:", err)
	}

	var wg sync.WaitGroup
	var errA, errB error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errA = sstore.Update(ctx, a.ID, wsConcurrentReorder, map[string]any{"position": float64(2)})
	}()
	go func() {
		defer wg.Done()
		_, errB = sstore.Update(ctx, b.ID, wsConcurrentReorder, map[string]any{"position": float64(1)})
	}()
	wg.Wait()

	for name, err := range map[string]error{"A": errA, "B": errB} {
		if err != nil && !errors.Is(err, apperrors.ErrConflict) {
			t.Fatalf("update %s: expected nil or ErrConflict, got %v (raw DB error leaked, not cleanly translated)", name, err)
		}
	}

	rows, err := db.QueryContext(ctx,
		`SELECT position, count(*) FROM stage WHERE pipeline_id=$1 AND archived_at IS NULL GROUP BY position HAVING count(*) > 1`,
		pl.ID)
	if err != nil {
		t.Fatal("post-check query:", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("found two live stages sharing a position after the concurrent swap — uniqueness violation window")
	}
	if err := rows.Err(); err != nil {
		t.Fatal("post-check rows:", err)
	}
}
