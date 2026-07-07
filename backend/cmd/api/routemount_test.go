package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// servedResources is the set of top-level contract resources this pruned
// skeleton tree actually mounts (buildMux registers only this slice; the full
// crm.yaml contract declares many more — invoices, leads, imports, exports,
// approvals, integrations, … — that are intentionally NOT wired here).
//
// The route-mount gate checks the FORWARD direction only for these resources:
// every contract operation under a served resource must be mounted with its
// declared method. This catches the "endpoint the FE expects but the backend
// forgot to route" class (t21) without a 66-entry deny-list of unimplemented
// resources. Adding a resource here (when the tree grows to serve it) is a
// visible, reviewed act.
//
// Limitation (documented for the operator): a resource mounted in routes.go but
// missing here is silently unchecked, and because crud() registers subtree
// patterns (/people/) most sub-paths resolve regardless of method — so the real
// teeth are on the method-specific mounts (auth, passports, members, deals
// custom verbs). A stronger "no orphan route" inverse check (every mounted
// pattern ∈ contract) needs ServeMux pattern introspection Go does not expose,
// so it is left as a follow-up.
var servedResources = map[string]bool{
	"workspaces": true, "auth": true, "me": true,
	"passports": true, "roles": true, "members": true,
	"people": true, "organizations": true, "deals": true,
	"pipelines": true, "stages": true, "partners": true,
	"relationships": true, "activities": true, "records": true,
}

// TestEveryServedContractOpIsRouted asserts every crm.yaml operation under a
// served resource resolves to a mounted route on the real mux, using the
// operation's declared HTTP method — without a DB connection. Route
// registration never touches the DB (store constructors just hold the *sql.DB
// handle), and mux.Handler resolves the pattern without executing the handler.
func TestEveryServedContractOpIsRouted(t *testing.T) {
	ops := loadContractOps(t)
	mux := buildTestMux(t)

	// Guard against a typo in servedResources: every entry must match at least
	// one contract path's top segment.
	seen := map[string]bool{}
	for _, op := range ops {
		seen[topSegment(op.path)] = true
	}
	for r := range servedResources {
		if !seen[r] {
			t.Errorf("servedResources has %q but no contract path starts with /%s — stale entry?", r, r)
		}
	}

	for _, op := range ops {
		if !servedResources[topSegment(op.path)] {
			continue // resource intentionally not wired in this tree
		}
		req := httptest.NewRequest(op.method, concretePath(op.path), nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Errorf("%s %s (%s) is NOT routed — mount it or remove it from the contract",
				op.method, op.path, op.id)
		}
	}
}

type contractOp struct{ method, path, id string }

func topSegment(p string) string {
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			return seg
		}
	}
	return ""
}

// concretePath turns /deals/{id}/advance into /deals/x/advance so a ServeMux
// wildcard/subtree pattern matches.
func concretePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, "{") {
			parts[i] = "x"
		}
	}
	return strings.Join(parts, "/")
}

// buildTestMux builds the real route mux with a nil DB handle. This is safe:
// route *registration* only stores the *sql.DB on the stores/middleware; the
// handle is dereferenced solely inside request handlers (e.g. RbacMiddleware's
// LoadRolePermissions), which mux.Handler resolves without executing. Using nil
// (rather than sql.Open) also keeps this in the unit lane — no real-infra open.
func buildTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	var db *sql.DB
	return buildMux(context.Background(), db, Config{}, nil)
}

// loadContractOps returns every (method, path, operationId) from crm.yaml.
func loadContractOps(t *testing.T) []contractOp {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "api", "crm.yaml"))
	if err != nil {
		t.Fatalf("read crm.yaml: %v", err)
	}
	// A path item mixes HTTP-method operations (mappings) with non-method keys
	// (parameters: a seq, summary/description: scalars), so decode the inner
	// values as raw nodes and pick only method keys.
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse crm.yaml: %v", err)
	}
	var ops []contractOp
	for path, item := range doc.Paths {
		for m, node := range item {
			mu := strings.ToUpper(m)
			switch mu {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				var o struct {
					//nolint:tagliatelle // "operationId" is the literal OpenAPI spec key (camelCase by the spec, not our choice)
					OperationID string `yaml:"operationId"`
				}
				_ = node.Decode(&o)
				ops = append(ops, contractOp{method: mu, path: path, id: o.OperationID})
			}
		}
	}
	if len(ops) == 0 {
		t.Fatal("no operations parsed from crm.yaml")
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].path < ops[j].path })
	return ops
}
