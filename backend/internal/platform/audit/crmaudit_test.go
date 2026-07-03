package crmaudit_test

import (
	"context"
	"testing"

	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

func TestEntryFromPrincipal_Human(t *testing.T) {
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "u1", TenantID: "ws1", IsAgent: false})
	e := crmaudit.EntryFromPrincipal(ctx, "create", "person", strptr("p1"), nil, map[string]any{"x": 1})
	if e.ActorType != "human" {
		t.Fatalf("actor_type = %q, want human", e.ActorType)
	}
	if e.ActorID != "u1" {
		t.Fatalf("actor_id = %q, want u1", e.ActorID)
	}
	if e.PassportID != nil {
		t.Fatalf("human must have nil passport_id, got %v", *e.PassportID)
	}
	if e.WorkspaceID != "ws1" {
		t.Fatalf("workspace = %q, want ws1", e.WorkspaceID)
	}
}

func TestEntryFromPrincipal_Agent(t *testing.T) {
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "human-1", TenantID: "ws1", IsAgent: true})
	e := crmaudit.EntryFromPrincipal(ctx, "update", "deal", strptr("d1"), nil, nil)
	if e.ActorType != "agent" {
		t.Fatalf("actor_type = %q, want agent", e.ActorType)
	}
	if e.OnBehalfOf == nil || *e.OnBehalfOf != "human-1" {
		t.Fatalf("agent must record on_behalf_of=granting human, got %v", e.OnBehalfOf)
	}
}

func TestEntryFromPrincipal_System(t *testing.T) {
	e := crmaudit.EntryFromPrincipal(context.Background(), "create", "person", nil, nil, nil)
	if e.ActorType != "system" || e.ActorID != "system" {
		t.Fatalf("no principal must be system/system, got %q/%q", e.ActorType, e.ActorID)
	}
	if e.PassportID != nil {
		t.Fatalf("system must have nil passport_id")
	}
}

func strptr(s string) *string { return &s }
