//go:build integration

package crmaudit_test

import (
	"context"
	"testing"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func TestWrite_OneRowPerMutation_Human(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	userID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, userID, wsID, "u"+userID+"@t.test", "U")

	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})
	// Call the seam directly: one Write call = exactly one audit row.
	entID := ids.New()
	auditID, err := crmaudit.Write(ctx, db,
		crmaudit.EntryFromPrincipal(ctx, "create", "person", &entID, nil, nil))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if auditID == "" {
		t.Fatal("expected non-empty auditID")
	}
	var n int
	mustQuery(t, db, &n, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action='create'`, entID)
	if n != 1 {
		t.Fatalf("expected exactly one audit row, got %d", n)
	}
	var actorType string
	var passport *string
	mustQuery2(t, db, &actorType, &passport, `SELECT actor_type, passport_id FROM audit_log WHERE entity_id=$1::uuid`, entID)
	if actorType != "human" {
		t.Fatalf("actor_type=%q want human", actorType)
	}
	if passport != nil {
		t.Fatalf("human must have NULL passport_id")
	}
}

func TestWrite_Agent_RecordsOnBehalfOf(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	userID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, userID, wsID, "u"+userID+"@t.test", "U")
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: true})
	id := ids.New()
	if _, err := crmaudit.Write(ctx, db, crmaudit.EntryFromPrincipal(ctx, "update", "deal", &id, nil, nil)); err != nil {
		t.Fatalf("write: %v", err)
	}
	var actorType string
	var obo *string
	mustQuery2(t, db, &actorType, &obo, `SELECT actor_type, on_behalf_of FROM audit_log WHERE entity_id=$1::uuid`, id)
	if actorType != "agent" || obo == nil || *obo != userID {
		t.Fatalf("agent attribution wrong: type=%q obo=%v", actorType, obo)
	}
}

func TestWrite_EmitsAuditAppended_Idempotent(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})
	entID := ids.New()
	e := crmaudit.Entry{
		WorkspaceID: wsID, ActorType: "system", ActorID: "system",
		Action: "create", EntityType: "person", EntityID: &entID,
	}
	auditID, err := crmaudit.Write(ctx, db, e)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	var topic string
	mustQuery(t, db, &topic, `SELECT topic FROM event_outbox WHERE entity_id=$1::uuid`, auditID)
	if topic != "audit.appended" {
		t.Fatalf("topic=%q want audit.appended", topic)
	}
	// Re-emit the SAME audit row via the REAL symbol (relay re-delivery path):
	// must be idempotent on audit_log_id (no second row).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.workspace_id',$1,true)`, wsID); err != nil {
		t.Fatal(err)
	}
	if err := crmaudit.EmitAuditAppended(ctx, tx, e, auditID); err != nil {
		t.Fatalf("re-emit: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var n int
	mustQuery(t, db, &n, `SELECT count(*) FROM event_outbox WHERE entity_id=$1::uuid AND topic='audit.appended'`, auditID)
	if n != 1 {
		t.Fatalf("audit.appended must be idempotent on audit_log_id, got %d rows", n)
	}
}

func TestWrite_EmissionFailure_DoesNotRollBackMutation(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	userID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, userID, wsID, "u"+userID+"@t.test", "U")
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})
	// Insert a person row directly (its own committed tx — models the domain
	// mutation that already committed before the audit write runs).
	pID := ids.New()
	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)
	mustExec(t, db, `INSERT INTO person (id,workspace_id,full_name,source,captured_by,version)
		VALUES ($1::uuid,$2::uuid,'Bob',$3,$4,1)`, pID, wsID, "human:"+userID, "human:"+userID)
	// Now a SEPARATE audit Write fails loudly (invalid action -> CHECK violation).
	// It must NOT roll back the already-committed person row.
	_, werr := crmaudit.Write(ctx, db, crmaudit.Entry{WorkspaceID: wsID, ActorType: "human", ActorID: userID, Action: "BOGUS_ACTION", EntityType: "person", EntityID: &pID})
	if werr == nil {
		t.Fatal("expected audit Write to fail loudly on invalid action")
	}
	var n int
	mustQuery(t, db, &n, `SELECT count(*) FROM person WHERE id=$1::uuid`, pID)
	if n != 1 {
		t.Fatalf("committed domain row must survive a failed audit write, count=%d", n)
	}
}
