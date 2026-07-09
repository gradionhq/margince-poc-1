package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

type eventTopicEmitter struct {
	emitted []struct {
		topic string
		id    string
	}
}

func (e *eventTopicEmitter) Emit(_ context.Context, _ crmapprovals.DBExec, topic, _ string, entityID string, _ json.RawMessage) error {
	e.emitted = append(e.emitted, struct {
		topic string
		id    string
	}{topic: topic, id: entityID})
	return nil
}

type eventTopicEffector struct{}

func (eventTopicEffector) Apply(context.Context, crmapprovals.DBExec, string, json.RawMessage) (string, error) {
	return "rollback", nil
}

func eventTopicTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://margince:margince@localhost:5432/margince_test?sslmode=disable"
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedWorkspaceEventTopic(t *testing.T, db *sql.DB) string {
	t.Helper()
	wsID := ids.New()
	if _, err := db.Exec(
		`INSERT INTO workspace (id, name, slug, base_currency) VALUES ($1::uuid,$2,$3,'EUR')`,
		wsID, "agents-"+wsID, "agents-"+wsID,
	); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	return wsID
}

func seedDealEventTopic(t *testing.T, db *sql.DB, wsID string) string {
	t.Helper()
	var pipelineID, stageID string
	if err := db.QueryRow(
		`INSERT INTO pipeline (workspace_id, name) VALUES ($1, $2) RETURNING id`,
		wsID, "agents-pipeline-"+ids.New(),
	).Scan(&pipelineID); err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if err := db.QueryRow(
		`INSERT INTO stage (workspace_id, pipeline_id, name, position) VALUES ($1, $2, 'Open', 1) RETURNING id`,
		wsID, pipelineID,
	).Scan(&stageID); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	var dealID string
	if err := db.QueryRow(
		`INSERT INTO deal (workspace_id, name, pipeline_id, stage_id, source, captured_by) VALUES ($1, $2, $3, $4, 'fixture', 'agent:overnight') RETURNING id`,
		wsID, "agents-deal-"+ids.New(), pipelineID, stageID,
	).Scan(&dealID); err != nil {
		t.Fatalf("seed deal: %v", err)
	}
	return dealID
}

func TestApplyGreen_EventTopic_FallsBackWhenEmpty(t *testing.T) {
	db := eventTopicTestDB(t)
	wsID := seedWorkspaceEventTopic(t, db)
	effector := eventTopicEffector{}
	emitter := &eventTopicEmitter{}
	p := domain.RoutedProposal{
		Proposal: domain.Proposal{
			WorkspaceID:  wsID,
			ActionType:   "close-date-auto-apply",
			TargetEntity: "deal:" + seedDealEventTopic(t, db, wsID),
			Effect:       json.RawMessage(`{}`),
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
	if len(emitter.emitted) != 1 || emitter.emitted[0].topic != app.TopicOvernightApplied {
		t.Fatalf("expected fallback topic %q, got %+v", app.TopicOvernightApplied, emitter.emitted)
	}
}

func TestApplyGreen_EventTopic_UsesProposalTopicWhenSet(t *testing.T) {
	db := eventTopicTestDB(t)
	wsID := seedWorkspaceEventTopic(t, db)
	effector := eventTopicEffector{}
	emitter := &eventTopicEmitter{}
	p := domain.RoutedProposal{
		Proposal: domain.Proposal{
			WorkspaceID:  wsID,
			ActionType:   "close-date-provisional-set",
			TargetEntity: "deal:" + seedDealEventTopic(t, db, wsID),
			Effect:       json.RawMessage(`{}`),
			EventTopic:   "deal.updated",
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
	if len(emitter.emitted) != 1 || emitter.emitted[0].topic != "deal.updated" {
		t.Fatalf("expected topic deal.updated, got %+v", emitter.emitted)
	}
}

func TestHandleDecided_EventTopic_DecodedFromPayload(t *testing.T) {
	db := eventTopicTestDB(t)
	wsID := seedWorkspaceEventTopic(t, db)
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

	emitter := &eventTopicEmitter{}
	effector := eventTopicEffector{}
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
