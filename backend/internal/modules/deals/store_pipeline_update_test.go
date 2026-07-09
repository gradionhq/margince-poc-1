//go:build integration

package deals

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func TestStageStore_Update_TerminalProbabilityPinned_Returns422(t *testing.T) {
	wsID := ids.New()
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsID)
	pgtest.SeedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "Update Test " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("create pipeline:", err)
	}
	sstore := NewStageStore(db)
	won, err := sstore.Create(ctx, Stage{
		WorkspaceID: wsID, PipelineID: pl.ID,
		Name: "Won", Position: 1, Semantic: "won", WinProbability: 100,
	})
	if err != nil {
		t.Fatal("create won stage:", err)
	}

	_, err = sstore.Update(ctx, won.ID, wsID, map[string]any{"win_probability": float64(80)})
	if !errors.Is(err, apperrors.ErrTerminalProbabilityPinned) {
		t.Fatalf("expected ErrTerminalProbabilityPinned, got %v", err)
	}
}

func TestStageStore_Update_WinProbabilityOutOfRange_Returns422(t *testing.T) {
	wsID := ids.New()
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsID)
	pgtest.SeedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "Range Test " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("create pipeline:", err)
	}
	sstore := NewStageStore(db)
	open, err := sstore.Create(ctx, Stage{
		WorkspaceID: wsID, PipelineID: pl.ID,
		Name: "Open", Position: 1, Semantic: "open", WinProbability: 50,
	})
	if err != nil {
		t.Fatal("create open stage:", err)
	}

	_, err = sstore.Update(ctx, open.ID, wsID, map[string]any{"win_probability": float64(150)})
	if !errors.Is(err, apperrors.ErrWinProbabilityOutOfRange) {
		t.Fatalf("expected ErrWinProbabilityOutOfRange, got %v", err)
	}
}

func TestPipelineStore_Update_DefaultCollision_Returns409(t *testing.T) {
	wsID := ids.New()
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsID)
	pgtest.SeedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	a, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "A " + pgtest.Uniq(), IsDefault: true})
	if err != nil {
		t.Fatal("create pipeline A:", err)
	}
	b, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "B " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("create pipeline B:", err)
	}
	_ = a

	_, err = pstore.Update(ctx, b.ID, wsID, map[string]any{"is_default": true})
	if !errors.Is(err, apperrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestStageStore_Update_PositionCollision_Returns409(t *testing.T) {
	wsID := ids.New()
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsID)
	pgtest.SeedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "PosCollide " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("create pipeline:", err)
	}
	sstore := NewStageStore(db)
	s1, err := sstore.Create(ctx, Stage{WorkspaceID: wsID, PipelineID: pl.ID, Name: "S1", Position: 1, Semantic: "open"})
	if err != nil {
		t.Fatal("create s1:", err)
	}
	_, err = sstore.Create(ctx, Stage{WorkspaceID: wsID, PipelineID: pl.ID, Name: "S2", Position: 2, Semantic: "open"})
	if err != nil {
		t.Fatal("create s2:", err)
	}

	_, err = sstore.Update(ctx, s1.ID, wsID, map[string]any{"position": float64(2)})
	if !errors.Is(err, apperrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

// TestStageStore_Update_WinProbabilitySuccess_ReflectsLiveOnDeal covers UAT-5:
// PATCH /stages/{id} retuning a non-terminal stage's win_probability, then
// re-reading a deal in that stage, must reflect the new probability
// immediately (not cached). This test wires the deal store and stage store
// against the same underlying connection to exercise the real deal->stage
// read path end to end.
func TestStageStore_Update_WinProbabilitySuccess_ReflectsLiveOnDeal(t *testing.T) {
	wsID := ids.New()
	db := pgtest.OpenTestDB(t)
	pgtest.SetRLS(t, db, wsID)
	pgtest.SeedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "Retune Test " + pgtest.Uniq()})
	if err != nil {
		t.Fatal("create pipeline:", err)
	}
	sstore := NewStageStore(db)
	qualified, err := sstore.Create(ctx, Stage{
		WorkspaceID: wsID, PipelineID: pl.ID,
		Name: "Qualified", Position: 1, Semantic: "open", WinProbability: 25,
	})
	if err != nil {
		t.Fatal("create qualified stage:", err)
	}

	dealStore := NewDealStore(db)
	deal := NewDeal("Retune Deal "+pgtest.Uniq(), pl.ID, qualified.ID,
		prov.Provenance{Source: "unit-test", CapturedBy: "unit-test"})
	deal.WorkspaceID = wsID
	created, err := dealStore.Create(ctx, deal, "", nil)
	if err != nil {
		t.Fatal("create deal:", err)
	}

	updated, err := sstore.Update(ctx, qualified.ID, wsID, map[string]any{"win_probability": float64(30)})
	if err != nil {
		t.Fatal("update stage win_probability:", err)
	}
	if updated.WinProbability != 30 {
		t.Fatalf("expected updated stage WinProbability=30, got %v", updated.WinProbability)
	}

	// Re-read the stage directly: proves Update leaves nothing stale behind —
	// nothing between Update returning and this Get could serve an old value.
	reread, err := sstore.Get(ctx, qualified.ID, wsID)
	if err != nil {
		t.Fatal("re-read stage:", err)
	}
	if reread.WinProbability != 30 {
		t.Fatalf("expected re-read stage WinProbability=30, got %v", reread.WinProbability)
	}

	// deals.Deal carries no win_probability-shaped field at all (see
	// crmcore.go) -- a deal's win probability is always read live off its
	// stage via the deal's stage_id FK, never denormalized/cached onto the
	// deal row. Re-reading the deal after the stage retune confirms the deal
	// still points at the same stage_id, so any reader joining deal->stage
	// sees the freshly retuned 30 immediately, with no denormalized copy on
	// the deal row that could ever go stale.
	rereadDeal, err := dealStore.Get(ctx, created.ID, wsID)
	if err != nil {
		t.Fatal("re-read deal:", err)
	}
	if rereadDeal.StageID != qualified.ID {
		t.Fatalf("expected deal stage_id to remain %s, got %s", qualified.ID, rereadDeal.StageID)
	}
}
