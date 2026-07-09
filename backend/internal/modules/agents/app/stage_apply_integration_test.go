//go:build integration

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
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
	crmapprovals "github.com/gradionhq/margince/backend/internal/modules/approvals"
	apperrors "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func testDB(t *testing.T) *sql.DB {
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

func seedWorkspace(t *testing.T, db *sql.DB) string {
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

// seedDeal inserts one minimal valid deal row and returns its id — used to
// prove GATE-AI-2 (zero domain-table writes pre-decision).
func seedDeal(t *testing.T, db *sql.DB, wsID string) string {
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

func dealSnapshot(t *testing.T, db *sql.DB, id string) (status string, version int64) {
	t.Helper()
	if err := db.QueryRow(
		`SELECT status, version FROM deal WHERE id = $1`,
		id,
	).Scan(&status, &version); err != nil {
		t.Fatalf("deal snapshot: %v", err)
	}
	return status, version
}

func TestStageProposal_ZeroDomainWritesBeforeDecision(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	dealID := seedDeal(t, db, wsID)
	beforeStatus, beforeVersion := dealSnapshot(t, db, dealID)

	repo := crmapprovals.NewRepository()
	conf := 0.85
	// RouteTier (Task 2) is the only production path to a tier — used here
	// too, rather than hand-setting RoutedProposal.Tier, to exercise the
	// real derivation ("close-deal" is a D4 name, so this always lands 🟡).
	p := app.RouteTier(domain.Proposal{
		WorkspaceID:  wsID,
		ActionType:   "close-deal", // a D4 name — deliberately the highest-stakes case
		TargetEntity: "deal:" + dealID,
		Effect:       json.RawMessage(`{"kind":"deal","id":"` + dealID + `","fields":{"status":"won"}}`),
		Evidence:     "fixture evidence",
		Confidence:   &conf,
		Source:       "fixture:source",
	})

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	itemID, stageErr := app.StageProposal(context.Background(), tx, repo, crmapprovals.Stage, p, json.RawMessage(`{"preview":true}`))
	if !errors.Is(stageErr, apperrors.ErrRequiresApproval) {
		_ = tx.Rollback()
		t.Fatalf("StageProposal: got %v, want ErrRequiresApproval", stageErr)
	}
	if itemID == "" {
		_ = tx.Rollback()
		t.Fatal("StageProposal: empty item id")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	afterStatus, afterVersion := dealSnapshot(t, db, dealID)
	if afterStatus != beforeStatus || afterVersion != beforeVersion {
		t.Fatalf("deal row changed pre-decision: before=(%s,%d) after=(%s,%d)", beforeStatus, beforeVersion, afterStatus, afterVersion)
	}

	item, err := repo.Get(context.Background(), db, itemID)
	if err != nil {
		t.Fatalf("get staged item: %v", err)
	}
	if item.ActionType != "overnight.close-deal" {
		t.Fatalf("ActionType = %q, want \"overnight.close-deal\"", item.ActionType)
	}
	if item.RequestedBy != app.ActorOvernight {
		t.Fatalf("RequestedBy = %q, want %q", item.RequestedBy, app.ActorOvernight)
	}
}

func TestApplyGreen_WritesAuditAndEvent(t *testing.T) {
	// Use a spy Effector + a spy EventEmitter (in-memory, no real domain
	// table for the 🟢 fixture kind) but a REAL crmaudit.WriteTx against
	// the real audit_log table, so the audit-row assertion is real.
	// audit_log.entity_id is uuid NOT NULL-shaped (nullable column, uuid
	// type), so the fixture entity id must itself be a real uuid, not an
	// arbitrary string like "fixture-1" — the audit row is real now that
	// ApplyGreen sets EntityID (ONA-T02 live-UAT fix).
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	conf := 0.95
	activityID := ids.New()
	// "log_link" is the one action type Task 2's tier router declares
	// TierGreen — routed for real here too, not hand-set.
	p := app.RouteTier(domain.Proposal{
		WorkspaceID:  wsID,
		ActionType:   "log_link",
		TargetEntity: "activity:" + activityID,
		Effect:       json.RawMessage(`{"link":"call-123"}`),
		Evidence:     "fixture evidence",
		Confidence:   &conf,
		Source:       "fixture:source",
	})
	effector := &spyEffector{rollbackHandle: "rb-1"}
	emitter := &spyEmitter{}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	handle, err := app.ApplyGreen(context.Background(), tx, effector, emitter, p)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("ApplyGreen: %v", err)
	}
	if handle != "rb-1" {
		t.Fatalf("rollback handle = %q, want rb-1", handle)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if !effector.called {
		t.Fatal("effector.Apply was never called")
	}
	if len(emitter.emitted) != 1 || emitter.emitted[0].topic != "overnight.applied" {
		t.Fatalf("expected exactly one overnight.applied emission, got %+v", emitter.emitted)
	}
	// entity_id must be just the id portion of TargetEntity (activityID),
	// never the whole "kind:id" composite ("activity:"+activityID) — a real
	// EventEmitter's event_outbox.entity_id column is uuid NOT NULL and
	// would reject the composite string.
	if got := emitter.emitted[0].entityID; got != activityID {
		t.Fatalf("emitted entityID = %q, want %q", got, activityID)
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE workspace_id = $1::uuid AND actor_id = $2`, wsID, app.ActorOvernight).Scan(&count); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("audit_log rows for agent:overnight = %d, want 1", count)
	}

	// ONA-T02 live-UAT fix: audit_log.entity_id must be populated (not NULL)
	// for every green-applied proposal — this ticket wires the first real
	// domain writes (close-date corrections) through this exact path.
	var gotEntityID sql.NullString
	if err := db.QueryRow(
		`SELECT entity_id FROM audit_log WHERE workspace_id = $1::uuid AND actor_id = $2`,
		wsID, app.ActorOvernight,
	).Scan(&gotEntityID); err != nil {
		t.Fatalf("select audit_log.entity_id: %v", err)
	}
	if !gotEntityID.Valid || gotEntityID.String != activityID {
		t.Fatalf("audit_log.entity_id = %+v, want valid uuid %q", gotEntityID, activityID)
	}
}

// spyEffector / spyEmitter: minimal test doubles, defined once here and
// reused by Task 4's executor_integration_test.go in the same package.
type spyEffector struct {
	called         bool
	rollbackHandle string
	err            error
}

func (s *spyEffector) Apply(_ context.Context, _ ports.DBExec, _ string, _ json.RawMessage) (string, error) {
	s.called = true
	return s.rollbackHandle, s.err
}

type emission struct {
	topic, entityID string
	payload         json.RawMessage
}

type spyEmitter struct{ emitted []emission }

func (s *spyEmitter) Emit(_ context.Context, _ ports.DBExec, topic, _, entityID string, payload json.RawMessage) error {
	s.emitted = append(s.emitted, emission{topic: topic, entityID: entityID, payload: payload})
	return nil
}
