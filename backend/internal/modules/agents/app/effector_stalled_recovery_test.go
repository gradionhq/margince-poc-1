package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
)

type stalledRecoveryLoggerCall struct {
	workspaceID string
	dealID      string
	subject     string
	body        string
	source      string
	capturedBy  string
}

type fakeActivityLogger struct {
	calls []stalledRecoveryLoggerCall
	id    string
	err   error
}

func (f *fakeActivityLogger) LogFollowUp(_ context.Context, _ crmapprovals.DBExec, workspaceID, dealID, subject, body, source, capturedBy string) (string, error) {
	f.calls = append(f.calls, stalledRecoveryLoggerCall{
		workspaceID: workspaceID,
		dealID:      dealID,
		subject:     subject,
		body:        body,
		source:      source,
		capturedBy:  capturedBy,
	})
	return f.id, f.err
}

func TestStalledRecoveryEffector_ApprovedSendUsesDraftAndReturnsRollbackHandle(t *testing.T) {
	logger := &fakeActivityLogger{id: "activity-123"}
	effector := app.StalledRecoveryEffector{Logger: logger}
	payload := json.RawMessage(mustJSON(map[string]any{
		"reason":               "no_reply_14_days",
		"evidence_activity_id": "act-9",
		"deal_id":              "9",
		"workspace_id":         "ws-9",
		"draft": map[string]any{
			"subject": "Checking in",
			"body":    "Hi, just following up.",
		},
	}))

	handle, err := effector.Apply(context.Background(), nil, "stalled_recovery", payload)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if handle != "activity-123" {
		t.Fatalf("rollback handle = %q, want activity-123", handle)
	}
	if len(logger.calls) != 1 {
		t.Fatalf("expected one logger call, got %d", len(logger.calls))
	}
	call := logger.calls[0]
	if call.workspaceID != "ws-9" || call.dealID != "9" {
		t.Fatalf("logger workspace/deal = %q/%q, want ws-9/9", call.workspaceID, call.dealID)
	}
	if call.subject != "Checking in" || call.body != "Hi, just following up." {
		t.Fatalf("logger subject/body = %q/%q", call.subject, call.body)
	}
	if call.source != app.ActorOvernight {
		t.Fatalf("logger source = %q, want %q", call.source, app.ActorOvernight)
	}
	if call.capturedBy != app.ActorOvernight {
		t.Fatalf("logger capturedBy = %q, want %q", call.capturedBy, app.ActorOvernight)
	}
}

func TestStalledRecoveryEffector_ApprovedFlagOnlyWithoutDraftIsNoOp(t *testing.T) {
	logger := &fakeActivityLogger{id: "activity-123"}
	effector := app.StalledRecoveryEffector{Logger: logger}
	payload := json.RawMessage(mustJSON(map[string]any{
		"reason":               "champion_quiet",
		"evidence_activity_id": "act-10",
		"deal_id":              "10",
		"workspace_id":         "ws-10",
	}))

	handle, err := effector.Apply(context.Background(), nil, "stalled_recovery", payload)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if handle != "" {
		t.Fatalf("rollback handle = %q, want empty for flag-only approval", handle)
	}
	if len(logger.calls) != 0 {
		t.Fatalf("expected no logger calls, got %d", len(logger.calls))
	}
}

func TestStalledRecoveryEffector_MalformedPayloadReturnsError(t *testing.T) {
	logger := &fakeActivityLogger{id: "activity-123"}
	effector := app.StalledRecoveryEffector{Logger: logger}
	handle, err := effector.Apply(context.Background(), nil, "stalled_recovery", json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected an error for a malformed payload")
	}
	if handle != "" {
		t.Fatalf("rollback handle = %q, want empty on malformed payload", handle)
	}
	if len(logger.calls) != 0 {
		t.Fatalf("expected no logger calls for a malformed payload, got %d", len(logger.calls))
	}
}
