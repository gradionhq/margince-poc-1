#!/usr/bin/env bash
# check-rls-store-path.sh — the deterministic teeth for the #1 source-quality finding.
#
# modules/directory (crm-core's successor, 1c restructure — task-3-brief.md)
# ran on a superuser pool that BYPASSES FORCE RLS, so a store statement that
# queries the bare pool (`s.db.QueryContext`/`ExecContext`/`QueryRowContext`) has zero
# tenant isolation — the `WHERE workspace_id=$` predicate is not a substitute (data-model
# §1.3, .ai/checklist/backend-invariants.md "RLS is engaged, not just intended"). Every
# per-workspace statement must run inside `withWorkspaceTx` (SET LOCAL ROLE margince_app +
# app.workspace_id) and address the tx, not the pool.
#
# GH-81 Task 6 (WS-E-b) dissolved modules/directory into 7 domain modules —
# organizations, activities, relationships, leads, partners, audithistory,
# datasourcebindings — and GH-81 Task 5 (WS-E-a, before it) moved every
# module's Postgres-touching store code into a <module>/adapters/
# subdirectory per the D6 shape convention. deals (already explicit here,
# unaffected by Task 6 itself) and people (transport-only before, in scope
# now since person store code came from directory) round the list out to 9.
# This list is intentionally the directory's-successors-plus-deals set, not
# every module under modules/ — approvals, gdpr, and identity are
# deliberately NOT scanned by this gate, before or after this fix: they are
# background sweeps and pre-tenant-context auth bootstrapping with different,
# deliberate cross-workspace/pre-tenant-context RLS characteristics, out of
# scope for this specific lint. A future new entity-module needs an explicit
# one-line addition to this array to get coverage — that's by design, so the
# gate's scope is always visible and reviewable rather than a silent glob.
#
# This lint fails if any non-test file under one of these modules' adapters/
# addresses `*.db.{Exec,Query,QueryRow}Context` directly. The sole sanctioned
# escape hatch for a genuinely cross-workspace/system query is a
# `// rls-exempt: <reason>` comment on the immediately-preceding line — use
# it sparingly.
#
# Heuristic assumption: it matches the package's uniform `<recv>.db.<Method>Context` shape.
# A store that renamed the pool field (e.g. `s.pool`) or used a non-Context method would slip
# past — keep the `.db` convention, or widen the pattern below if that ever changes.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# directory's 2026-07 successor modules (GH-81 Task 6, WS-E-b) plus deals —
# see header comment above for why this is a hardcoded list, not a glob.
modules=(
  organizations
  activities
  relationships
  leads
  partners
  audithistory
  datasourcebindings
  deals
  people
)
dirs=()
for m in "${modules[@]}"; do
  dirs+=("$root/backend/internal/modules/$m/adapters")
done

# One awk pass over every non-test adapters/ .go file; prev resets per file (FNR==1).
files="$(for d in "${dirs[@]}"; do find "$d" -maxdepth 1 -name '*.go' ! -name '*_test.go'; done | sort)"
violations="$(echo "$files" | xargs awk '
  FNR == 1 { prev = "" }
  $0 ~ /[A-Za-z_][A-Za-z0-9_]*\.db\.(ExecContext|QueryContext|QueryRowContext)/ {
    if (prev !~ /\/\/[[:space:]]*rls-exempt:/) {
      line = $0; sub(/^[[:space:]]+/, "", line)
      printf "%s:%d: %s\n", FILENAME, FNR, line
    }
  }
  { prev = $0 }
')"

if [ -n "$violations" ]; then
  echo "FAIL — directory-successor module (or deals) adapters/ store statements addressing the superuser pool directly (RLS bypassed):"
  echo "$violations"
  echo
  echo "Route each through withWorkspaceTx (SET LOCAL ROLE margince_app + app.workspace_id),"
  echo "or, for a genuinely cross-workspace query, add a '// rls-exempt: <reason>' line above it."
  exit 1
fi

echo "PASS — crm-core RLS store-path OK (no direct superuser-pool statements)"
