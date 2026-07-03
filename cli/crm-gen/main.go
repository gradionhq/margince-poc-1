// Command crm-gen is the source-customization generator (ADR-0002 / blueprint
// 08). WP0 implements the `field` recipe: add a typed column to a core object
// across the layers, starting with a real, sequenced migration pair. The
// remaining layers (struct field, OpenAPI fragment, regenerated types) are
// emitted as the recipe to apply — the contract lives in the foundation repo
// (referenced, source of truth), so the field's OpenAPI fragment is added there
// and `make gen-types` re-generates the Go + TS types.
//
// Usage:
//
//	crm-gen field <table> <column> <sql-type> [go-type]
//
// Example:
//
//	crm-gen field person nickname "text" string
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const migrationsDir = "backend/migrations"

var seqRe = regexp.MustCompile(`^(\d{6})_`)

// commands maps each subcommand to its generator. genConnector's optional
// baseDir (test-only) is dropped here; the CLI always scaffolds under ".".
var commands = map[string]func([]string) error{
	"field":     genField,
	"object":    genObject,
	"workflow":  genWorkflow,
	"connector": func(args []string) error { return genConnector(args) },
	"tool":      genTool,
	"report":    genReport,
	"manifests": genManifests,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	gen, ok := commands[cmd]
	if !ok {
		usage()
		os.Exit(2)
	}
	if err := gen(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "crm-gen %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `crm-gen — Margince source-customization generator (ADR-0002)

Usage:
  crm-gen field     <table> <column> <sql-type> [go-type]
  crm-gen object    <Name>
  crm-gen workflow  <name>
  crm-gen connector <name>
  crm-gen tool      <name>
  crm-gen report    <name>
  crm-gen manifests

Examples:
  crm-gen field person nickname "text" string
  crm-gen object InvoiceItem
  crm-gen workflow deal_stalled
  crm-gen connector gmail
  crm-gen tool search_contacts
  crm-gen report pipeline_velocity
  crm-gen manifests

field:     Writes a sequenced migration pair under `+migrationsDir+`/ and prints the
           remaining recipe steps (struct field, OpenAPI fragment, make gen-types).
object:    Scaffolds a new custom CRM object with migration pair (RLS pre-wired),
           Go domain struct, and test stubs under backend/internal/modules/directory/custom/<name>/.
workflow:  Scaffolds a self-registering workflow handler in crm/crm-capture/workflows/.
connector: Scaffolds a self-registering capture connector in crm/crm-capture/connectors/.
tool:      Scaffolds a governed MCP tool (default tier: Green) in crm/crm-agents/tools/.
report:    Scaffolds a read-only compiled report query in backend/internal/modules/directory/reports/.
manifests: Scans connector/workflow/tool dirs and regenerates cmd/server/imports_gen.go.
`)
}

func genField(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("need <table> <column> <sql-type> [go-type]")
	}
	table, column, sqlType := args[0], args[1], args[2]
	goType := "string"
	if len(args) >= 4 {
		goType = args[3]
	}
	if !identRe.MatchString(table) || !identRe.MatchString(column) {
		return fmt.Errorf("table/column must be snake_case identifiers")
	}

	seq, err := nextSeq(migrationsDir)
	if err != nil {
		return err
	}
	base := fmt.Sprintf("%s_add_%s_%s", seq, table, column)
	up := filepath.Join(migrationsDir, base+".up.sql")
	down := filepath.Join(migrationsDir, base+".down.sql")

	upSQL := fmt.Sprintf("-- crm gen field: add %s.%s\nALTER TABLE %s ADD COLUMN %s %s;\n",
		table, column, table, column, sqlType)
	downSQL := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;\n", table, column)
	// up/down derive from table+column, both validated against identRe
	// (^[a-z][a-z0-9_]*$) above, so the joined path cannot escape migrationsDir.
	//nolint:gosec // G703: path components are identRe-validated, no traversal possible.
	if err := os.WriteFile(up, []byte(upSQL), filePerm); err != nil {
		return err
	}
	//nolint:gosec // G703: path components are identRe-validated, no traversal possible.
	if err := os.WriteFile(down, []byte(downSQL), filePerm); err != nil {
		return err
	}

	printSummary(fmt.Sprintf("✓ wrote %s\n✓ wrote %s\n\n", up, down) +
		"Recipe — finish the field across the layers (blueprint 08):\n" +
		fmt.Sprintf("  1. Struct: add to the %s domain type:\n        %s %s\n", table, goFieldName(column), goType) +
		fmt.Sprintf("  2. Contract: add to backend/api/crm.yaml schema %s.properties:\n        %s: { %s }\n",
			schemaName(table), column, openAPIFor(sqlType)) +
		"  3. Regenerate typed Go + TS:  make gen-types\n" +
		"  4. Apply + verify:            make migrate-up && make gen-types-check && make check\n" +
		fmt.Sprintf("  5. Add a contract round-trip assertion for %s.%s.\n", table, column))
	return nil
}

var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func nextSeq(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w (run from the repo root)", dir, err)
	}
	highest := 0
	for _, e := range entries {
		if m := seqRe.FindStringSubmatch(e.Name()); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > highest {
				highest = n
			}
		}
	}
	return fmt.Sprintf("%06d", highest+1), nil
}

func goFieldName(col string) string {
	parts := strings.Split(col, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// schemaName maps a table to its OpenAPI schema (singular PascalCase).
func schemaName(table string) string { return goFieldName(table) }

// openAPIFor maps a coarse SQL type to an OpenAPI property hint.
func openAPIFor(sqlType string) string {
	s := strings.ToLower(sqlType)
	switch {
	case strings.HasPrefix(s, "bigint"), strings.HasPrefix(s, "integer"), strings.HasPrefix(s, "smallint"):
		return `type: integer`
	case strings.HasPrefix(s, "boolean"):
		return `type: boolean`
	case strings.HasPrefix(s, "timestamptz"), strings.HasPrefix(s, "date"):
		return `type: string, format: date-time`
	case strings.HasPrefix(s, "numeric"), strings.HasPrefix(s, "double"):
		return `type: number`
	default:
		return `type: string`
	}
}
