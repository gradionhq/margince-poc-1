//go:build integration

package crmaudit_test

import (
	"context"
	"testing"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

func TestAgentTrace_IngestAndResolveFromAudit_NoRunner(t *testing.T) {
	db := testDB(t)
	wsID := ids.New()
	userID := ids.New()
	mustExec(t, db, `INSERT INTO workspace (id,name,slug,base_currency) VALUES ($1::uuid,$2,$3,'EUR')`, wsID, "w"+wsID, "w"+wsID)
	mustExec(t, db, `INSERT INTO app_user (id,workspace_id,email,display_name) VALUES ($1::uuid,$2::uuid,$3,$4)`, userID, wsID, "u"+userID+"@t.test", "Agent")
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: userID, TenantID: wsID, IsAgent: true})
	// produce an audit row to link to (no runner involved)
	entID := ids.New()
	auditID, err := crmaudit.Write(ctx, db, crmaudit.EntryFromPrincipal(ctx, "update", "deal", &entID, nil, nil))
	if err != nil {
		t.Fatalf("audit write: %v", err)
	}
	te := crmaudit.TraceEntry{
		WorkspaceID:   wsID,
		TraceID:       "trace-xyz",
		ActorID:       userID,
		AuditLogID:    &auditID,
		Inputs:        map[string]any{"prompt": "advance the deal"},
		ToolCalls:     []crmaudit.ToolCall{{Name: "advance_stage", Args: map[string]any{"to": "won"}}},
		Outputs:       map[string]any{"ok": true},
		ApprovalState: "auto",
	}
	id, err := crmaudit.Ingest(ctx, db, te)
	if err != nil || id == "" {
		t.Fatalf("ingest: id=%q err=%v", id, err)
	}
	got, err := crmaudit.ResolveFromAudit(ctx, db, auditID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.TraceID != "trace-xyz" || len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "advance_stage" {
		t.Fatalf("resolved trace mismatch: %+v", got)
	}
}
