#!/usr/bin/env bash
# check-rls-store-path.sh — the deterministic teeth for the #1 source-quality finding.
#
# Two violation shapes, both meaning a statement against a tenant table can run without
# FORCE RLS actually engaged (data-model §1.3):
#   (1) a store addresses the bare superuser pool directly via a receiver field
#       (`s.db.{Exec,Query,QueryRow}Context`), bypassing withWorkspaceTx/WithWorkspaceTx
#       entirely — the historical modules/directory + modules/deals check.
#   (2) a file sets the app.workspace_id GUC (`set_config('app.workspace_id'`) without ever
#       switching to margince_app in the same file — the GH-209 WS-A shape (gdpr, approvals,
#       platform/audit, identity/transport all had this before the fix: an open tx that set
#       the GUC but never ran `SET LOCAL ROLE margince_app`, so the superuser role stayed
#       active and FORCE RLS never engaged).
#
# Widened (GH-209 WS-A#3) to scan every backend/internal/modules/* subtree and
# backend/internal/platform/audit, recursively — previously only modules/directory and
# modules/deals at maxdepth 1. platform/database itself is intentionally excluded: it is
# the seam's own implementation and legitimately contains both statements together.
#
# The sole sanctioned escape hatch for a genuinely cross-workspace/system statement is a
# `// rls-exempt: <reason>` comment on the immediately-preceding line — use it sparingly.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

dirs=("$root/backend/internal/modules"/*/ "$root/backend/internal/platform/audit")

files="$(for d in "${dirs[@]}"; do find "$d" -name '*.go' ! -name '*_test.go'; done | sort -u)"

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

# Pattern 2: a set_config('app.workspace_id' with no SET LOCAL ROLE margince_app anywhere
# earlier in the same function (per-function via a `^func ` boundary reset).
violations2="$(echo "$files" | xargs awk '
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
