package mcp

import "context"

// runReportTool is the governed MCP declaration for run_report (tier=green).
// Execution is via the POST /reports/{report} HTTP operation (runReport).
type runReportTool struct{}

func init() { Register(&runReportTool{}) }

func (*runReportTool) Name() string { return "run_report" }
func (*runReportTool) Tier() Tier   { return Green }

// Invoke acknowledges the governed-tool registration. V1 execution surface is
// the runReport HTTP operation; this stub satisfies the Tool interface.
func (*runReportTool) Invoke(_ context.Context, _ []byte) ([]byte, error) {
	return []byte(`{"message":"use POST /reports/{report} to execute run_report"}`), nil
}
