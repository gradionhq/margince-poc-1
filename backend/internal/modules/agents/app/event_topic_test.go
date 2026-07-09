//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// testDB, seedWorkspace, and seedDeal (used below) are defined once in
// stage_apply_integration_test.go and reused here — this file is the same
// app_test package, so there is no symbol collision to dodge. spyEffector
// and spyEmitter (also defined there) already satisfy ports.Effector and
// crmapprovals.EventEmitter with the exact shape these tests need
// (topic/entityID capture), so they are reused too instead of file-local
// doubles.

func TestApplyGreen_EventTopic(t *testing.T) {
	cases := []struct {
		name       string
		actionType string
		eventTopic string
		wantTopic  string
	}{
		{
			name:       "FallsBackWhenEmpty",
			actionType: "close-date-auto-apply",
			eventTopic: "",
			wantTopic:  app.TopicOvernightApplied,
		},
		{
			name:       "UsesProposalTopicWhenSet",
			actionType: "close-date-provisional-set",
			eventTopic: "deal.updated",
			wantTopic:  "deal.updated",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			wsID := seedWorkspace(t, db)
			effector := &spyEffector{rollbackHandle: "rollback"}
			emitter := &spyEmitter{}
			p := domain.RoutedProposal{
				Proposal: domain.Proposal{
					WorkspaceID:  wsID,
					ActionType:   tc.actionType,
					TargetEntity: "deal:" + seedDeal(t, db, wsID),
					Effect:       json.RawMessage(`{}`),
					EventTopic:   tc.eventTopic,
				},
				Tier: mcp.TierGreen,
			}

			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			if _, err := app.ApplyGreen(context.Background(), tx, effector, emitter, p); err != nil {
				_ = tx.Rollback()
				t.Fatalf("ApplyGreen: %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
			if len(emitter.emitted) != 1 || emitter.emitted[0].topic != tc.wantTopic {
				t.Fatalf("expected topic %q, got %+v", tc.wantTopic, emitter.emitted)
			}
		})
	}
}

func TestHandleDecided_EventTopic_DecodedFromPayload(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	itemID := ""

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin stage: %v", err)
	}
	itemID, err = crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "overnight.close-date-provisional-set",
		RequestedBy: app.ActorOvernight,
		Payload:     json.RawMessage(`{"deal_id":"d1","event_topic":"deal.updated"}`),
	})
	if !errors.Is(err, apperrors.ErrRequiresApproval) {
		_ = tx.Rollback()
		t.Fatalf("stage: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}

	emitter := &spyEmitter{}
	effector := &spyEffector{rollbackHandle: "rollback"}
	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx2, repo, effector, emitter, app.DecidedEventPayload{Decision: "approved", ItemID: itemID}); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("HandleDecided: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit handle: %v", err)
	}
	if len(emitter.emitted) != 1 || emitter.emitted[0].topic != "deal.updated" {
		t.Fatalf("expected topic deal.updated from the decoded payload, got %+v", emitter.emitted)
	}
}
