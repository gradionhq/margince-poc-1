//go:build integration

package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	_ "github.com/lib/pq" // registers the "postgres" driver for database/sql.Open

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// stageStalledRecoveryApproval stages one "overnight.stalled_recovery" item and
// fails the test unless the stage returns the expected ErrRequiresApproval —
// extracted to keep TestHandleDecided_ApprovedRecoverySendUsesFetchedPayloadAndProvenance
// under the cognitive-complexity gate.
func stageStalledRecoveryApproval(t *testing.T, db *sql.DB, repo crmapprovals.Repository, wsID string, payload json.RawMessage) string {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin stage: %v", err)
	}
	itemID, err := crmapprovals.Stage(context.Background(), tx, repo, crmapprovals.StageInput{
		WorkspaceID: wsID,
		ActionType:  "overnight.stalled_recovery",
		RequestedBy: app.ActorOvernight,
		Payload:     payload,
	})
	if !errors.Is(err, apperrors.ErrRequiresApproval) {
		_ = tx.Rollback()
		t.Fatalf("stage: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit stage: %v", err)
	}
	return itemID
}

// decideStalledRecoveryApproval drives app.HandleDecided for one staged item —
// same extraction rationale as stageStalledRecoveryApproval above.
func decideStalledRecoveryApproval(t *testing.T, db *sql.DB, repo crmapprovals.Repository, effector app.StalledRecoveryEffector, emitter ports.EventEmitter, itemID string) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin handle: %v", err)
	}
	if err := app.HandleDecided(context.Background(), tx, repo, effector, emitter, app.DecidedEventPayload{Decision: "approved", ItemID: itemID}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("HandleDecided: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit handle: %v", err)
	}
}

func TestHandleDecided_ApprovedRecoverySendUsesFetchedPayloadAndProvenance(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()
	logger := &fakeActivityLogger{id: "activity-789"}
	effector := app.StalledRecoveryEffector{Logger: logger}
	emitter := &spyEmitter{}

	payloadA := json.RawMessage(`{"reason":"no_reply_14_days","evidence_activity_id":"act-1","deal_id":"1","workspace_id":"` + wsID + `","draft":{"subject":"First draft","body":"first body"}}`)
	payloadB := json.RawMessage(`{"reason":"no_reply_14_days","evidence_activity_id":"act-1","deal_id":"1","workspace_id":"` + wsID + `","draft":{"subject":"Edited draft","body":"edited body"}}`)

	itemIDA := stageStalledRecoveryApproval(t, db, repo, wsID, payloadA)
	itemIDB := stageStalledRecoveryApproval(t, db, repo, wsID, payloadB)
	decideStalledRecoveryApproval(t, db, repo, effector, emitter, itemIDA)
	decideStalledRecoveryApproval(t, db, repo, effector, emitter, itemIDB)

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

	// Both crmapprovals.Stage (action="capture", no rollback_handle key) and
	// HandleDecided (action="update", carries rollback_handle) write an
	// audit_log row for the same workspace_id/actor_id/entity_id (Stage's
	// RequestedBy is ActorOvernight here too) — filter on action="update" to
	// deterministically select HandleDecided's own row, not Stage's.
	var rollbackA, rollbackB string
	if err := db.QueryRow(
		`SELECT after->>'rollback_handle' FROM audit_log WHERE workspace_id = $1::uuid AND actor_id = $2 AND entity_id = $3::uuid AND action = 'update'`,
		wsID, app.ActorOvernight, itemIDA,
	).Scan(&rollbackA); err != nil {
		t.Fatalf("query rollback A: %v", err)
	}
	if rollbackA != "activity-789" {
		t.Errorf("audit rollback_handle for item A = %q, want activity-789 (the logged activity id)", rollbackA)
	}
	if err := db.QueryRow(
		`SELECT after->>'rollback_handle' FROM audit_log WHERE workspace_id = $1::uuid AND actor_id = $2 AND entity_id = $3::uuid AND action = 'update'`,
		wsID, app.ActorOvernight, itemIDB,
	).Scan(&rollbackB); err != nil {
		t.Fatalf("query rollback B: %v", err)
	}
	if rollbackB != "activity-789" {
		t.Errorf("audit rollback_handle for item B = %q, want activity-789", rollbackB)
	}
}

func TestHandleDecided_StalledRecoveryDraftlessPayloadNoOps(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	repo := crmapprovals.NewRepository()

	payload := json.RawMessage(`{"reason":"no_reply_14_days","evidence_activity_id":"act-c","deal_id":"deal-c","workspace_id":"` + wsID + `","draft":null}`)
	itemID := stageStalledRecoveryApproval(t, db, repo, wsID, payload)

	logger := &fakeActivityLogger{id: "should-never-be-logged"}
	decideStalledRecoveryApproval(t, db, repo, app.StalledRecoveryEffector{Logger: logger}, &spyEmitter{}, itemID)

	if len(logger.calls) != 0 {
		t.Fatalf("expected Logger.LogFollowUp to never be called for a draft-less flag, got %+v", logger.calls)
	}
}
