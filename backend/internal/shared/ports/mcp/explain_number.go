package mcp

import "context"

// explainNumberTool is the governed MCP declaration for explain_number (tier=green).
// Execution is via the GET /reports/derivations/{handle} HTTP operation (resolveDerivation).
type explainNumberTool struct{}

func init() { Register(&explainNumberTool{}) }

func (*explainNumberTool) Name() string { return "explain_number" }
func (*explainNumberTool) Tier() Tier   { return Green }

// Invoke acknowledges the governed-tool registration. Execution surface is the
// resolveDerivation HTTP operation; this stub satisfies the Tool interface.
func (*explainNumberTool) Invoke(_ context.Context, _ []byte) ([]byte, error) {
	return []byte(`{"message":"use GET /reports/derivations/{handle} to execute explain_number"}`), nil
}
