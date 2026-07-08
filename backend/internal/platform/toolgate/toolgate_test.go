package toolgate_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/gradionhq/margince/backend/internal/platform/toolgate"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// fakeVerifier is a single-use-consuming approvalsport.Verifier stand-in —
// no DB, so these tests exercise toolgate.Enforce itself, not a copy of its
// logic.
type fakeVerifier struct {
	mu        sync.Mutex
	consumed  map[string]bool
	callCount int
	lastWant  approvalsport.Binding
}

func (f *fakeVerifier) VerifyAndConsume(_ context.Context, token string, want approvalsport.Binding) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	f.lastWant = want
	if f.consumed == nil {
		f.consumed = map[string]bool{}
	}
	if f.consumed[token] {
		return errs.ErrApprovalTokenInvalid
	}
	f.consumed[token] = true
	return nil
}

func TestToolgate_YellowTierAgentRequiresToken(t *testing.T) {
	tool := mcp.GeneratedTool{OperationID: "mergePerson", Verb: "merge_records", RecordType: "person", Tier: mcp.TierYellow}
	agent := crmctx.Principal{UserID: "agent-1", TenantID: "ws-1", IsAgent: true}
	diffFields := map[string]any{"person_id": "p1", "target_id": "p2"}
	fv := &fakeVerifier{}

	if err := toolgate.Enforce(context.Background(), agent, fv, tool, "ws-1", diffFields, nil, ""); !errors.Is(err, toolgate.ErrApprovalRequired) {
		t.Fatalf("no token: expected ErrApprovalRequired, got %v", err)
	}
	if fv.callCount != 0 {
		t.Fatalf("verifier must not be called when the token header is empty, got %d calls", fv.callCount)
	}

	if err := toolgate.Enforce(context.Background(), agent, fv, tool, "ws-1", diffFields, nil, "tok-1"); err != nil {
		t.Fatalf("valid token: expected success, got %v", err)
	}
	if fv.lastWant.Tool != "merge_records" || fv.lastWant.WorkspaceID != "ws-1" {
		t.Fatalf("verifier received unexpected binding: %+v", fv.lastWant)
	}

	if err := toolgate.Enforce(context.Background(), agent, fv, tool, "ws-1", diffFields, nil, "tok-1"); err == nil {
		t.Fatal("replayed token: expected an error, got nil")
	}
}

func TestToolgate_HumanBypassesApproval(t *testing.T) {
	tool := mcp.GeneratedTool{OperationID: "mergePerson", Verb: "merge_records", RecordType: "person", Tier: mcp.TierYellow}
	human := crmctx.Principal{UserID: "human-1", TenantID: "ws-1", IsAgent: false}
	fv := &fakeVerifier{}

	if err := toolgate.Enforce(context.Background(), human, fv, tool, "ws-1", map[string]any{"person_id": "p1"}, nil, ""); err != nil {
		t.Fatalf("human principal must self-approve with no token, got %v", err)
	}
	if fv.callCount != 0 {
		t.Fatalf("verifier must not be called for a human principal, got %d calls", fv.callCount)
	}
}

func TestToolgate_DynamicTierResolvesFromArgs(t *testing.T) {
	const resolverName = "toolgate_test_target_stage_semantic"
	toolgate.RegisterResolver(resolverName, func(args map[string]any) mcp.RiskTier {
		to, _ := args["to_semantic"].(string)
		if to == "won" || to == "lost" {
			return mcp.TierYellow
		}
		return mcp.TierGreen
	})
	tool := mcp.GeneratedTool{OperationID: "advanceDeal", Verb: "advance_deal", RecordType: "deal", Tier: mcp.TierDynamic, Resolver: resolverName}
	agent := crmctx.Principal{UserID: "agent-1", TenantID: "ws-1", IsAgent: true}
	fv := &fakeVerifier{}

	// Target stage semantic "open" resolves green — no token needed even for an agent.
	if err := toolgate.Enforce(context.Background(), agent, fv, tool, "ws-1", map[string]any{"to_semantic": "open"}, nil, ""); err != nil {
		t.Fatalf("open transition: expected green (no gate), got %v", err)
	}
	if fv.callCount != 0 {
		t.Fatalf("verifier must not be called for a green-resolved transition, got %d calls", fv.callCount)
	}

	// Target stage semantic "won" resolves yellow — agent requires a token.
	if err := toolgate.Enforce(context.Background(), agent, fv, tool, "ws-1", map[string]any{"to_semantic": "won"}, nil, ""); !errors.Is(err, toolgate.ErrApprovalRequired) {
		t.Fatalf("won transition: expected ErrApprovalRequired, got %v", err)
	}
}
