//go:build integration

package crmaudit_test

import (
	"context"
	"testing"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// The static `audit-coherence` gate (scripts/check-audit-action-coherence.sh)
// proves crm.yaml's enum == the Postgres CHECK. It cannot prove the *third* leg:
// that the live DB actually admits every action the contract — and therefore the
// runtime — can emit. A drift there is the latent bug this branch fixes: code
// wrote action="send_email" and the CHECK rejected it at runtime.
//
// These matrices close that leg. They are driven by the *generated* enum consts
// (crm.yaml -> oapi-codegen), so the list is the contract at compile time: drop a
// value from crm.yaml and the const vanishes -> these tables fail to compile.

// allContractActions enumerates AuditLogEntry.action. Keep in sync with the
// generated AuditLogEntryAction consts (compiler enforces names exist).
var allContractActions = []types.AuditLogEntryAction{
	types.AuditLogEntryActionAdvanceStage,
	types.AuditLogEntryActionAnonymize,
	types.AuditLogEntryActionApprove,
	types.AuditLogEntryActionArchive,
	types.AuditLogEntryActionAssign,
	types.AuditLogEntryActionCapture,
	types.AuditLogEntryActionConsentGrant,
	types.AuditLogEntryActionConsentWithdraw,
	types.AuditLogEntryActionCreate,
	types.AuditLogEntryActionDisqualify,
	types.AuditLogEntryActionErase,
	types.AuditLogEntryActionExpired,
	types.AuditLogEntryActionExport,
	types.AuditLogEntryActionGenerate,
	types.AuditLogEntryActionImport,
	types.AuditLogEntryActionLogin,
	types.AuditLogEntryActionMerge,
	types.AuditLogEntryActionModify,
	types.AuditLogEntryActionParameterize,
	types.AuditLogEntryActionPause,
	types.AuditLogEntryActionPromote,
	types.AuditLogEntryActionPublish,
	types.AuditLogEntryActionRecordShare,
	types.AuditLogEntryActionRecordUnshare,
	types.AuditLogEntryActionReject,
	types.AuditLogEntryActionRestore,
	types.AuditLogEntryActionSendEmail,
	types.AuditLogEntryActionUpdate,
}

var allContractActorTypes = []types.AuditLogEntryActorType{
	types.AuditLogEntryActorTypeHuman,
	types.AuditLogEntryActorTypeAgent,
	types.AuditLogEntryActorTypeSystem,
	types.AuditLogEntryActorTypeConnector,
}

// TestAuditCHECK_AcceptsEveryContractAction: every action the contract defines
// must be writable against the real audit_log_action_check. One subtest per
// action so a regression names the exact value the CHECK rejects.
func TestAuditCHECK_AcceptsEveryContractAction(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	userID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, userID, wsID, "u"+userID+"@t.test", "U")
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID})

	for _, action := range allContractActions {
		t.Run(string(action), func(t *testing.T) {
			entID := ids.New()
			if _, err := crmaudit.Write(ctx, db,
				crmaudit.EntryFromPrincipal(ctx, string(action), "person", &entID, nil, nil)); err != nil {
				t.Fatalf("audit_log_action_check rejected contract action %q at write time: %v", action, err)
			}
			var n int
			mustQuery(t, db, &n, `SELECT count(*) FROM audit_log WHERE entity_id=$1::uuid AND action=$2`, entID, string(action))
			if n != 1 {
				t.Fatalf("action %q: expected one audit row, got %d", action, n)
			}
		})
	}
}

// TestAuditCHECK_AcceptsEveryContractActorType: the actor_type CHECK this branch
// also reconciles must admit every contract actor_type. Entry is built directly
// because EntryFromPrincipal only ever yields human/agent/system.
func TestAuditCHECK_AcceptsEveryContractActorType(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	ctx := crmctx.With(context.Background(), crmctx.Principal{TenantID: wsID})

	for _, at := range allContractActorTypes {
		t.Run(string(at), func(t *testing.T) {
			entID := ids.New()
			e := crmaudit.Entry{
				WorkspaceID: wsID, ActorType: string(at), ActorID: "actor",
				Action: "create", EntityType: "person", EntityID: &entID,
			}
			if _, err := crmaudit.Write(ctx, db, e); err != nil {
				t.Fatalf("audit_log_actor_type_check rejected contract actor_type %q at write time: %v", at, err)
			}
		})
	}
}
