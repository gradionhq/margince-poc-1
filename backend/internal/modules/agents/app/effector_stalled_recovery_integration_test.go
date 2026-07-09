//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

func TestHandleDecided_ApprovedRecoverySendUsesFetchedPayloadAndProvenance(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	logger := &fakeActivityLogger{id: "activity-789"}
	effector := app.StalledRecoveryEffector{Logger: logger}
	emitter := &spyEmitter{}

	payloadA := json.RawMessage(`{"reason":"no_reply_14_days","evidence_activity_id":"act-1","deal_id":"1","workspace_id":"` + wsID + `","draft":{"subject":"First draft","body":"first body"}}`)
	payloadB := json.RawMessage(`{"reason":"no_reply_14_days","evidence_activity_id":"act-1","deal_id":"1","workspace_id":"` + wsID + `","draft":{"subject":"Edited draft","body":"edited body"}}`)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin stage A: %v", err)
	}
	itemIDA, err := crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "overnight.stalled_recovery",
		RequestedBy: app.ActorOvernight,
		Payload:     payloadA,
	})
	if !errors.Is(err, apperrors.ErrRequiresApproval) {
		_ = tx.Rollback()
		t.Fatalf("stage A: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage A: %v", err)
	}

	tx2, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin stage B: %v", err)
	}
	itemIDB, err := crmapprovals.Stage(context.Background(), tx2, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "overnight.stalled_recovery",
		RequestedBy: app.ActorOvernight,
		Payload:     payloadB,
	})
	if !errors.Is(err, apperrors.ErrRequiresApproval) {
		_ = tx2.Rollback()
		t.Fatalf("stage B: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit stage B: %v", err)
	}

	tx3, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle A: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx3, repo, effector, emitter, app.DecidedEventPayload{Decision: "approved", ItemID: itemIDA}); err != nil {
		_ = tx3.Rollback()
		t.Fatalf("HandleDecided A: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("commit handle A: %v", err)
	}

	tx4, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle B: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx4, repo, effector, emitter, app.DecidedEventPayload{Decision: "approved", ItemID: itemIDB}); err != nil {
		_ = tx4.Rollback()
		t.Fatalf("HandleDecided B: %v", err)
	}
	if err := tx4.Commit(); err != nil {
		t.Fatalf("commit handle B: %v", err)
	}

	if len(logger.calls) != 2 {
		t.Fatalf("expected 2 logger calls, got %d", len(logger.calls))
	}
	if logger.calls[0].subject != "First draft" || logger.calls[1].subject != "Edited draft" {
		t.Fatalf("logger subjects = %+v, want fetched payloads in order", []string{logger.calls[0].subject, logger.calls[1].subject})
	}
	if logger.calls[0].body != "first body" || logger.calls[1].body != "edited body" {
		t.Fatalf("logger bodies = %+v, want fetched payloads in order", []string{logger.calls[0].body, logger.calls[1].body})
	}
	if logger.calls[1].workspaceID != wsID || logger.calls[1].dealID != "1" {
		t.Fatalf("logger provenance = %+v, want workspace %s / deal 1", logger.calls[1], wsID)
	}
	if len(emitter.emitted) != 2 {
		t.Fatalf("expected 2 overnight.applied emissions, got %d", len(emitter.emitted))
	}
}
