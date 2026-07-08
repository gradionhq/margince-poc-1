package main

import "testing"

const mcpToolsFixtureYAML = `
paths:
  /people/{id}:
    get:
      operationId: getPerson
      # a comment mentioning x-mcp-tool: must never be treated as data
      x-mcp-tool: { verb: read_record, record_type: person, tier: green }
  /deals/{id}/advance:
    post:
      operationId: advanceDeal
      x-mcp-tool: { verb: advance_deal, record_type: deal, tier: dynamic, tier_resolver: target_stage_semantic, yellow_when: 'target stage semantic in (won, lost)' }
`

func TestParseMCPTools(t *testing.T) {
	tools, err := parseMCPTools([]byte(mcpToolsFixtureYAML))
	if err != nil {
		t.Fatalf("parseMCPTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("parseMCPTools() found %d tools, want 2 (the comment-only mention must not count): %+v", len(tools), tools)
	}

	byOp := map[string]mcpToolEntry{}
	for _, tl := range tools {
		byOp[tl.OperationID] = tl
	}

	get, ok := byOp["getPerson"]
	if !ok {
		t.Fatalf("missing getPerson entry: %+v", tools)
	}
	if get.Verb != "read_record" || get.RecordType != "person" || get.Tier != "green" || get.Resolver != "" {
		t.Errorf("getPerson = %+v, want verb=read_record record_type=person tier=green resolver=\"\"", get)
	}

	adv, ok := byOp["advanceDeal"]
	if !ok {
		t.Fatalf("missing advanceDeal entry: %+v", tools)
	}
	if adv.Verb != "advance_deal" || adv.RecordType != "deal" || adv.Tier != "dynamic" || adv.Resolver != "target_stage_semantic" {
		t.Errorf("advanceDeal = %+v, want verb=advance_deal record_type=deal tier=dynamic resolver=target_stage_semantic", adv)
	}
}

func TestParseMCPToolsRequiresOperationID(t *testing.T) {
	const missingOpID = `
paths:
  /widgets:
    get:
      x-mcp-tool: { verb: read_record, record_type: widget, tier: green }
`
	if _, err := parseMCPTools([]byte(missingOpID)); err == nil {
		t.Fatal("parseMCPTools() with no operationId should error, got nil")
	}
}

// TestRenderMCPToolsGoIsDeterministic is the gen-mcp-tools-check idempotency
// proof: parsing the same spec twice goes through Go's randomized map
// iteration internally, so byte-identical output across two independent
// parses proves the final OperationID sort — not incidental map order — is
// what makes regeneration diff-clean.
func TestRenderMCPToolsGoIsDeterministic(t *testing.T) {
	const spec = `
paths:
  /zzz:
    get:
      operationId: zListWidgets
      x-mcp-tool: { verb: search_records, record_type: widget, tier: green }
  /aaa:
    get:
      operationId: aGetWidget
      x-mcp-tool: { verb: read_record, record_type: widget, tier: green }
  /mmm:
    post:
      operationId: mCreateWidget
      x-mcp-tool: { verb: create_record, record_type: widget, tier: yellow }
`
	toolsA, err := parseMCPTools([]byte(spec))
	if err != nil {
		t.Fatalf("parseMCPTools() error = %v", err)
	}
	toolsB, err := parseMCPTools([]byte(spec))
	if err != nil {
		t.Fatalf("parseMCPTools() error = %v", err)
	}

	first, err := renderMCPToolsGo(toolsA)
	if err != nil {
		t.Fatalf("renderMCPToolsGo() error = %v", err)
	}
	second, err := renderMCPToolsGo(toolsB)
	if err != nil {
		t.Fatalf("renderMCPToolsGo() error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("renderMCPToolsGo() not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
