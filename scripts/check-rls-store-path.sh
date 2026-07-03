#!/usr/bin/env bash
# check-rls-store-path.sh — the deterministic teeth for the #1 source-quality finding.
#
# modules/directory (crm-core's successor, 1c restructure — task-3-brief.md)
# runs on a superuser pool that BYPASSES FORCE RLS, so a store statement that
# queries the bare pool (`s.db.QueryContext`/`ExecContext`/`QueryRowContext`) has zero
# tenant isolation — the `WHERE workspace_id=$` predicate is not a substitute (data-model
# §1.3, .ai/checklist/backend-invariants.md "RLS is engaged, not just intended"). Every
# per-workspace statement must run inside `withWorkspaceTx` (SET LOCAL ROLE margince_app +
# app.workspace_id) and address the tx, not the pool. withWorkspaceTx (and PersonStore,
# which uses it) stayed in modules/directory rather than splitting into a standalone
# platform/database package — see task-3-report.md's mapping-deviations section: the
# helper is shared, unexported, same-package with every spine store, and extracting it
# would have forced those helpers to export solely to cross the new package boundary.
#
# This lint fails if any non-test modules/directory file addresses
# `*.db.{Exec,Query,QueryRow}Context` directly. The sole sanctioned escape hatch for a
# genuinely cross-workspace/system query is a `// rls-exempt: <reason>` comment on the
# immediately-preceding line — use it sparingly.
#
# Heuristic assumption: it matches the package's uniform `<recv>.db.<Method>Context` shape.
# A store that renamed the pool field (e.g. `s.pool`) or used a non-Context method would slip
# past — keep the `.db` convention, or widen the pattern below if that ever changes.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# modules/deals (T10) holds the pipeline/stage store, split out of
# modules/directory — scan both so the RLS-bypass gate keeps covering
# pipeline/stage after the move.
dirs=("$root/backend/internal/modules/directory" "$root/backend/internal/modules/deals")

# One awk pass over every non-test modules/directory .go file; prev resets per file (FNR==1).
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
  echo "FAIL — modules/directory or modules/deals store statements addressing the superuser pool directly (RLS bypassed):"
  echo "$violations"
  echo
  echo "Route each through withWorkspaceTx (SET LOCAL ROLE margince_app + app.workspace_id),"
  echo "or, for a genuinely cross-workspace query, add a '// rls-exempt: <reason>' line above it."
  exit 1
fi

echo "PASS — crm-core RLS store-path OK (no direct superuser-pool statements)"
