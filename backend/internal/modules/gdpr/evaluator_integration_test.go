//go:build integration

package crmgdpr_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/riverqueue/river"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// seedPipelineStageAndDeal creates a pipeline, stage, and deal; returns dealID.
// Caller must set app.workspace_id GUC before calling.
func seedPipelineStageAndDeal(t *testing.T, db *sql.DB, wsID, status string) string {
	t.Helper()
	pipelineID := ids.New()
	stageID := ids.New()
	dealID := ids.New()

	mustExec(t, db,
		`INSERT INTO pipeline (id, workspace_id, name, is_default)
		 VALUES ($1::uuid, $2::uuid, 'Test Pipeline', true)`,
		pipelineID, wsID)
	mustExec(t, db,
		`INSERT INTO stage (id, workspace_id, pipeline_id, name, position)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, 'Test Stage', 1)`,
		stageID, wsID, pipelineID)

	// updated_at backdated in the INSERT — trg_deal_touch resets it on UPDATE.
	if status == "lost" {
		mustExec(t, db,
			`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, status, lost_reason, closed_at, source, captured_by, updated_at)
			 VALUES ($1::uuid, $2::uuid, 'Test Deal', $3::uuid, $4::uuid, 'lost', 'budget', now(), 'test', 'test', now() - INTERVAL '1826 days')`,
			dealID, wsID, pipelineID, stageID)
	} else {
		mustExec(t, db,
			`INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, status, source, captured_by)
			 VALUES ($1::uuid, $2::uuid, 'Test Deal', $3::uuid, $4::uuid, $5, 'test', 'test')`,
			dealID, wsID, pipelineID, stageID, status)
	}
	return dealID
}

// TestRetentionEvaluator_AnonymizesOverAgeLead seeds an over-age unconverted lead,
// runs the RetentionWorker, and asserts the lead is anonymized with one audit row.
func TestRetentionEvaluator_AnonymizesOverAgeLead(t *testing.T) {
	db := testDB(t)
	wsID, _ := seedWorkspaceAndPerson(t, db)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Seed updated_at backdated directly in the INSERT — the BEFORE UPDATE
	// trg_lead_touch trigger would clobber updated_at back to now() on any UPDATE.
	leadID := ids.New()
	mustExec(t, db,
		`INSERT INTO lead (id, workspace_id, full_name, email, status, source, captured_by, updated_at)
		 VALUES ($1::uuid, $2::uuid, 'Alice Lead', 'alice@example.com', 'new', 'test', 'test', now() - INTERVAL '400 days')`,
		leadID, wsID)

	mustExec(t, db,
		`INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
		 VALUES ($1::uuid, 'lead', 'unconverted', 365, 'anonymize')
		 ON CONFLICT DO NOTHING`, wsID)

	w := crmgdpr.NewRetentionWorker(db)
	if err := w.Work(context.Background(), &river.Job[crmgdpr.RetentionSweepArgs]{}); err != nil {
		t.Fatalf("Work: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	var fullName, email *string
	if err := db.QueryRow(
		`SELECT full_name, email FROM lead WHERE id=$1::uuid`, leadID,
	).Scan(&fullName, &email); err != nil {
		t.Fatalf("scan lead: %v", err)
	}
	if fullName != nil {
		t.Errorf("lead.full_name: want NULL after anonymize, got %q", *fullName)
	}
	if email != nil {
		t.Errorf("lead.email: want NULL after anonymize, got %q", *email)
	}

	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='lead' AND entity_id=$1::uuid AND action='update'`,
		leadID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit rows for anonymized lead: want 1, got %d", auditCount)
	}
}

// TestRetentionEvaluator_SkipsLegalHold seeds an over-age lead with legal_hold=true.
// Verifies the lead is unchanged with no audit row.
func TestRetentionEvaluator_SkipsLegalHold(t *testing.T) {
	db := testDB(t)
	wsID, _ := seedWorkspaceAndPerson(t, db)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Over-age AND legal_hold=true so the skip is attributable to the hold, not age.
	// Backdate via INSERT — trg_lead_touch resets updated_at on UPDATE.
	heldID := ids.New()
	mustExec(t, db,
		`INSERT INTO lead (id, workspace_id, full_name, email, status, source, captured_by, legal_hold, updated_at)
		 VALUES ($1::uuid, $2::uuid, 'Held Lead', 'held@example.com', 'new', 'test', 'test', true, now() - INTERVAL '400 days')`,
		heldID, wsID)

	mustExec(t, db,
		`INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
		 VALUES ($1::uuid, 'lead', 'unconverted', 365, 'anonymize')
		 ON CONFLICT DO NOTHING`, wsID)

	w := crmgdpr.NewRetentionWorker(db)
	if err := w.Work(context.Background(), &river.Job[crmgdpr.RetentionSweepArgs]{}); err != nil {
		t.Fatalf("Work: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	var fullName *string
	if err := db.QueryRow(
		`SELECT full_name FROM lead WHERE id=$1::uuid`, heldID,
	).Scan(&fullName); err != nil {
		t.Fatalf("scan held lead: %v", err)
	}
	if fullName == nil || *fullName != "Held Lead" {
		t.Errorf("held lead.full_name: want 'Held Lead' (untouched), got %v", fullName)
	}

	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='lead' AND entity_id=$1::uuid`,
		heldID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("audit count for held lead: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("audit rows for held lead: want 0, got %d", auditCount)
	}
}

// TestRetentionEvaluator_ArchivesOverAgeLostDeal seeds an over-age lost deal,
// runs the RetentionWorker, and asserts archived_at is set with one audit row.
func TestRetentionEvaluator_ArchivesOverAgeLostDeal(t *testing.T) {
	db := testDB(t)
	wsID, _ := seedWorkspaceAndPerson(t, db)
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	dealID := seedPipelineStageAndDeal(t, db, wsID, "lost")

	mustExec(t, db,
		`INSERT INTO retention_policy (workspace_id, object_type, category, retain_days, action)
		 VALUES ($1::uuid, 'deal', 'lost', 1825, 'archive')
		 ON CONFLICT DO NOTHING`, wsID)

	w := crmgdpr.NewRetentionWorker(db)
	if err := w.Work(context.Background(), &river.Job[crmgdpr.RetentionSweepArgs]{}); err != nil {
		t.Fatalf("Work: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	var archivedAt *time.Time
	if err := db.QueryRow(
		`SELECT archived_at FROM deal WHERE id=$1::uuid`, dealID,
	).Scan(&archivedAt); err != nil {
		t.Fatalf("scan deal: %v", err)
	}
	if archivedAt == nil {
		t.Error("deal.archived_at: want non-NULL after archive action")
	}

	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE entity_type='deal' AND entity_id=$1::uuid AND action='archive'`,
		dealID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("audit count for deal: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit rows for archived deal: want 1, got %d", auditCount)
	}
}
