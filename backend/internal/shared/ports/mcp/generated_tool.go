package mcp

// GeneratedTool is one `x-mcp-tool` annotation lifted from backend/api/crm.yaml
// by `crm-gen mcp-tools` (ADR-0002 codegen surface). It is hand-written data
// shape only — the actual table of tools lives in the generated tools_gen.go,
// keeping this file untouched by regeneration.
type GeneratedTool struct {
	OperationID string
	Verb        string
	RecordType  string
	Tier        RiskTier
	Resolver    string
}
