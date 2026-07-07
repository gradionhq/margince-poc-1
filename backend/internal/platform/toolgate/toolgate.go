// Package toolgate is the one tool-boundary admission gate every
// agent-invoked governed tool call site goes through (WS-D-b, AC-D2): it
// resolves a tool's effective risk tier from the crm-gen-generated
// mcp.GeneratedTool table and, for an agent principal on a non-green tier,
// requires and consumes a single-use X-Approval-Token. It depends only on
// the Tier-0 mcp/approvalsport/crmctx/ids seams (D9) — never on
// modules/approvals directly, which would recreate the exact
// platform-module→domain-module edge AC-E6 removes elsewhere (rejected at
// plan review as structurally identical to httpserver→identity).
package toolgate

import (
	"context"
	"errors"
	"sync"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	approvalsport "github.com/gradionhq/margince/backend/internal/shared/ports/approvals"
	"github.com/gradionhq/margince/backend/internal/shared/ports/mcp"
)

// ErrApprovalRequired is returned when an agent principal calls a non-green
// tool with no X-Approval-Token at all (API-ERR-10) — distinct from
// whatever error the Verifier returns for an invalid/expired/replayed token,
// so HTTP call sites can map each to its own problem+json code.
var ErrApprovalRequired = errors.New("toolgate: approval required")

var (
	resolversMu sync.RWMutex
	resolvers   = map[string]func(args map[string]any) mcp.RiskTier{}
)

// RegisterResolver registers fn as the dynamic-tier resolver for the
// mcp.GeneratedTool.Resolver name — called once per resolver at composition
// (cmd/api for production wiring). Re-registering the same name overwrites
// the prior resolver.
func RegisterResolver(name string, fn func(args map[string]any) mcp.RiskTier) {
	resolversMu.Lock()
	defer resolversMu.Unlock()
	resolvers[name] = fn
}

// effectiveTier resolves tool's tier for this call: static from tool.Tier,
// or via the registered resolver keyed by tool.Resolver for the dynamic
// case. Mirrors mcp.ToolSpec.ResolveTier's 🟡 floor invariant — an
// unregistered resolver, or one that returns anything but a clean
// TierGreen, floors to TierYellow, never escapes to TierGreen by omission.
func effectiveTier(tool mcp.GeneratedTool, args map[string]any) mcp.RiskTier {
	if tool.Tier != mcp.TierDynamic {
		return tool.Tier
	}
	resolversMu.RLock()
	fn := resolvers[tool.Resolver]
	resolversMu.RUnlock()
	if fn != nil && fn(args) == mcp.TierGreen {
		return mcp.TierGreen
	}
	return mcp.TierYellow
}

// Enforce is the one tool-boundary admission check every governed-tool call
// site makes (AC-D2). A human principal's direct call is itself the
// approval (self-approving, no token ever required). An agent principal on
// a green tool passes freely. An agent principal on a yellow (static or
// dynamically-resolved) tool must present a single-use X-Approval-Token
// bound to this exact (workspace, tool verb, diff, target version) —
// verified and consumed via verifier, the Tier-0 approvalsport.Verifier
// seam (never modules/approvals directly, D9).
func Enforce(ctx context.Context, principal crmctx.Principal, verifier approvalsport.Verifier, tool mcp.GeneratedTool, wsID string, diffFields map[string]any, targetVersion *int64, tokenHeader string) error {
	tier := effectiveTier(tool, diffFields)
	if !principal.IsAgent || tier == mcp.TierGreen {
		return nil
	}
	if tokenHeader == "" {
		return ErrApprovalRequired
	}
	diffHash := approvalsport.HashDiff(diffFields)
	return verifier.VerifyAndConsume(ctx, tokenHeader, approvalsport.Binding{
		WorkspaceID: wsID, Tool: tool.Verb, DiffHash: diffHash, TargetVersion: targetVersion,
	})
}
