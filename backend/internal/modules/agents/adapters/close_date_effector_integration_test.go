//go:build integration

package adapters_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/deals"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

func TestCloseDateEffector_Apply_UpdatesExpectedCloseDateWithIfMatch(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := seedPipeline(t, db, wsID, []stageSpec{{name: "Open", position: 1, semantic: "open", winProb: 50}})
	dealID := seedOpenDeal(t, db, wsID, pipelineID, stages[0].id)

	store := deals.NewDealStore(db)
	before, err := store.Get(context.Background(), dealID, wsID)
	if err != nil {
		t.Fatalf("get before: %v", err)
	}

	effector := adapters.NewCloseDateEffector(store)
	payload, err := json.Marshal(map[string]any{
		"deal_id":          dealID,
		"workspace_id":     wsID,
		"if_match":         before.Version,
		"new_close_date":   "2026-08-01",
		"prior_close_date": nil,
		"prior_version":    before.Version,
		"event_topic":      "deal.updated",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	handle, err := effector.Apply(context.Background(), nil, "close-date-auto-apply", payload)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if handle == "" {
		t.Fatal("expected a non-empty rollback handle")
	}

	after, err := store.Get(context.Background(), dealID, wsID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if after.ExpectedCloseDate == nil || after.ExpectedCloseDate.Format("2006-01-02") != "2026-08-01" {
		t.Fatalf("expected_close_date = %v, want 2026-08-01", after.ExpectedCloseDate)
	}
	if after.Version != before.Version+1 {
		t.Fatalf("version = %d, want %d", after.Version, before.Version+1)
	}
}

func TestCloseDateEffector_Apply_IfMatchZero_SkipsVersionCheck(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := seedPipeline(t, db, wsID, []stageSpec{{name: "Open", position: 1, semantic: "open", winProb: 50}})
	dealID := seedOpenDeal(t, db, wsID, pipelineID, stages[0].id)

	effector := adapters.NewCloseDateEffector(deals.NewDealStore(db))
	payload, err := json.Marshal(map[string]any{
		"deal_id":        dealID,
		"workspace_id":   wsID,
		"if_match":       0,
		"new_close_date": "2026-09-15",
		"prior_version":  999,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := effector.Apply(context.Background(), nil, "close-date-confirm-request", payload); err != nil {
		t.Fatalf("Apply with if_match=0: %v", err)
	}

	after, err := deals.NewDealStore(db).Get(context.Background(), dealID, wsID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if after.ExpectedCloseDate == nil || after.ExpectedCloseDate.Format("2006-01-02") != "2026-09-15" {
		t.Fatalf("expected_close_date = %v, want 2026-09-15", after.ExpectedCloseDate)
	}
}

func TestCloseDateEffector_Apply_VersionSkew_Rejected(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	pipelineID, stages := seedPipeline(t, db, wsID, []stageSpec{{name: "Open", position: 1, semantic: "open", winProb: 50}})
	dealID := seedOpenDeal(t, db, wsID, pipelineID, stages[0].id)

	effector := adapters.NewCloseDateEffector(deals.NewDealStore(db))
	payload, err := json.Marshal(map[string]any{
		"deal_id":        dealID,
		"workspace_id":   wsID,
		"if_match":       999,
		"new_close_date": "2026-08-01",
		"prior_version":  999,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	_, err = effector.Apply(context.Background(), nil, "close-date-provisional-set", payload)
	if !errors.Is(err, errs.ErrVersionSkew) {
		t.Fatalf("expected ErrVersionSkew, got %v", err)
	}
}
