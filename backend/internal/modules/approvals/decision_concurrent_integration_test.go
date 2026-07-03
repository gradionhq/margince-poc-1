//go:build integration

package crmapprovals_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/lib/pq"

	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/ports/datasource"
)

// countingSor counts datasource.Update calls across goroutines; everything else delegates.
type countingSor struct {
	*testSorProvider
	updates atomic.Int64
}

func (c *countingSor) Update(ctx context.Context, in datasource.UpdateInput) (datasource.EntityRef, error) {
	c.updates.Add(1)
	return c.testSorProvider.Update(ctx, in)
}

// TestDecider_Approve_ExactlyOnce_Concurrent_RealDB is the real-Postgres twin of the
// unit-level concurrent test: two approvers race the SAME pending item in two separate
// txs. The conditional `UPDATE … WHERE status='pending'` is the row lock — Postgres
// serialises the two UPDATEs, so the loser sees status!=pending (RowsAffected=0) and
// refuses BEFORE exec. Proves the irreversible datasource action fires exactly once against
// real UPDATE-lock semantics, not a fake mutex.
func TestDecider_Approve_ExactlyOnce_Concurrent_RealDB(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	sorSpy := &countingSor{testSorProvider: &testSorProvider{}}
	decider := crmapprovals.Decider{Repo: repo, Datasource: sorSpy}

	agentID := "agent:concurrent-" + ids.New()
	payload := json.RawMessage(`{"kind":"person","id":"abc123","fields":{"email":"new@example.com"},"source":"mcp","captured_by":"` + agentID + `"}`)

	// Stage one pending item and commit it.
	stageTx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin stage tx: %v", err)
	}
	itemID, stageErr := crmapprovals.Stage(context.Background(), stageTx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "update_record",
		RequestedBy: agentID,
		Payload:     payload,
	})
	if !errors.Is(stageErr, errs.ErrRequiresApproval) {
		_ = stageTx.Rollback()
		t.Fatalf("Stage: %v", stageErr)
	}
	if err := stageTx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}

	// Race two approvers, each in its own tx.
	approve := func() error {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`SELECT set_config('app.workspace_id', $1, true)`, wsID); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := decider.Approve(context.Background(), tx, itemID, "human:racer"); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}

	var wg sync.WaitGroup
	errsCh := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errsCh <- approve()
		}()
	}
	close(start)
	wg.Wait()
	close(errsCh)

	var wins, losses int
	for err := range errsCh {
		if err == nil {
			wins++
		} else {
			losses++
		}
	}
	if wins != 1 || losses != 1 {
		t.Fatalf("concurrent approve: wins=%d losses=%d, want exactly 1 win / 1 loss", wins, losses)
	}
	if got := sorSpy.updates.Load(); got != 1 {
		t.Fatalf("datasource.Update fired %d times under a concurrent approve race, want exactly 1", got)
	}
}
