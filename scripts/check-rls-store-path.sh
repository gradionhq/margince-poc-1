#!/usr/bin/env bash
# check-rls-store-path.sh — the deterministic teeth for the #1 source-quality finding.
#
# Two violation shapes, both meaning a statement against a tenant table can run without
# FORCE RLS actually engaged (data-model §1.3):
#   (1) a store addresses the bare superuser pool directly via a receiver field
#       (`s.db.{Exec,Query,QueryRow}Context`), bypassing withWorkspaceTx/WithWorkspaceTx
#       entirely — the historical modules/directory + modules/deals check.
#
#       GH-81 Task 6 (WS-E-b) dissolved modules/directory into 7 domain modules —
#       organizations, activities, relationships, leads, partners, audithistory,
#       datasourcebindings — and GH-81 Task 5 (WS-E-a, before it) moved every
#       module's Postgres-touching store code into a <module>/adapters/
#       subdirectory per the D6 shape convention. deals (already explicit here,
#       unaffected by Task 6 itself) and people (transport-only before, in scope
#       now since person store code came from directory) round the list out to 9.
#       This list is intentionally the directory's-successors-plus-deals set, not
#       every module under modules/ — approvals, gdpr, and identity are
#       deliberately NOT scanned by this pattern, before or after this fix: they are
#       background sweeps and pre-tenant-context auth bootstrapping with different,
#       deliberate cross-workspace/pre-tenant-context RLS characteristics, out of
#       scope for this specific pattern. A future new entity-module needs an explicit
#       one-line addition to this array to get coverage — that's by design, so the
#       gate's scope is always visible and reviewable rather than a silent glob.
#   (2) a file sets the app.workspace_id GUC (`set_config('app.workspace_id'`) without ever
#       switching to margince_app in the same file — the GH-209 WS-A shape (gdpr, approvals,
#       platform/audit, identity/transport all had this before the fix: an open tx that set
#       the GUC but never ran `SET LOCAL ROLE margince_app`, so the superuser role stayed
#       active and FORCE RLS never engaged).
#
#       Widened (GH-209 WS-A#3) to scan every backend/internal/modules/* subtree and
#       backend/internal/platform/audit, recursively — deliberately NOT restricted to the
#       9-module list above, since this pattern's known historical bypass sites (gdpr,
#       approvals, identity/transport) are exactly the modules Pattern 1 excludes.
#       platform/database itself is intentionally excluded: it is the seam's own
#       implementation and legitimately contains both statements together.
#
# The sole sanctioned escape hatch for a genuinely cross-workspace/system statement is a
# `// rls-exempt: <reason>` comment on the immediately-preceding line — use it sparingly.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Pattern 1 scan set: directory's 2026-07 successor modules (GH-81 Task 6, WS-E-b) plus
# deals and people — see header comment above for why this is a hardcoded list, not a glob.
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

# Pattern 1: bare receiver-field pool access (e.g. s.db.ExecContext(...)).
violations1="$(echo "$files" | xargs awk '
  FNR == 1 { prev = "" }
  $0 ~ /[A-Za-z_][A-Za-z0-9_]*\.db\.(ExecContext|QueryContext|QueryRowContext)/ {
    if (prev !~ /\/\/[[:space:]]*rls-exempt:/) {
      line = $0; sub(/^[[:space:]]+/, "", line)
      printf "%s:%d: %s\n", FILENAME, FNR, line
    }
  }
  { prev = $0 }
')"

# Pattern 2 scan set: every module (recursively) plus platform/audit — the GH-209 WS-A#3
# widening; platform/database itself is excluded (see header comment).
dirs2=("$root/backend/internal/modules"/*/ "$root/backend/internal/platform/audit")
files2="$(for d in "${dirs2[@]}"; do find "$d" -name '*.go' ! -name '*_test.go'; done | sort -u)"

# Pattern 2: a set_config('app.workspace_id' with no SET LOCAL ROLE margince_app anywhere
# earlier in the same function (per-function via a `^func ` boundary reset).
violations2="$(echo "$files2" | xargs awk '
  FNR == 1 { prev = ""; sawRole = 0 }
  /^func / { sawRole = 0 }
  /SET LOCAL ROLE margince_app/ { sawRole = 1 }
  $0 ~ /set_config\(.app\.workspace_id./ {
    if (!sawRole && prev !~ /\/\/[[:space:]]*rls-exempt:/) {
      line = $0; sub(/^[[:space:]]+/, "", line)
      printf "%s:%d: %s\n", FILENAME, FNR, line
    }
  }
  { prev = $0 }
')"

violations="$(printf '%s\n%s\n' "$violations1" "$violations2" | grep -v '^$' || true)"

if [ -n "$violations" ]; then
  echo "FAIL — a module or platform/audit statement can run with RLS not actually engaged:"
  echo "$violations"
  echo
  echo "Route each through platform/database.WithWorkspaceTx / SetWorkspaceScope (SET LOCAL"
  echo "ROLE margince_app + app.workspace_id), or, for a genuinely cross-workspace query, add"
  echo "a '// rls-exempt: <reason>' line above it."
  exit 1
fi

echo "PASS — RLS store-path OK (no bare-pool statements, no orphaned workspace GUC sets)"
