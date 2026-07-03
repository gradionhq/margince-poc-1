//go:build integration

package deals

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func TestStageStore_Update_TerminalProbabilityPinned_Returns422(t *testing.T) {
	wsID := ids.New()
	db := openTestDB(t)
	setRLS(t, db, wsID)
	seedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "Update Test " + uniq()})
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
	db := openTestDB(t)
	setRLS(t, db, wsID)
	seedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "Range Test " + uniq()})
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
	db := openTestDB(t)
	setRLS(t, db, wsID)
	seedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	a, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "A " + uniq(), IsDefault: true})
	if err != nil {
		t.Fatal("create pipeline A:", err)
	}
	b, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "B " + uniq()})
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
	db := openTestDB(t)
	setRLS(t, db, wsID)
	seedWorkspace(t, db, wsID)
	ctx := context.Background()

	pstore := NewPipelineStore(db)
	pl, err := pstore.Create(ctx, Pipeline{WorkspaceID: wsID, Name: "PosCollide " + uniq()})
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
