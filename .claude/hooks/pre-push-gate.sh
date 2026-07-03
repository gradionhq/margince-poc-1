#!/usr/bin/env bash
# PreToolUse(Bash) gate: block `git push` until the cheap, deterministic checks
# the disabled CI jobs would run pass for the current HEAD. These run for real
# here (real teeth — they can't be faked). The expensive craftsmanship taste
# review is NOT gated here: re-reviewing the whole cumulative branch diff on
# every push is redundant. It runs once at branch-finish (before the PR — see
# AGENTS.md ## Craftsmanship) and CI's craftsmanship job backstops it on the PR.
# Exit 2 blocks the tool call and feeds stderr back to the agent.
#
# The script self-guards on `git push` (the case statement below) so it is safe
# to run standalone; the settings.json `if: "Bash(git push*)"` is just an
# optimization to avoid spawning it on every Bash call.
set -euo pipefail

input="$(cat)"
cmd="$(printf '%s' "$input" | python3 -c 'import sys, json; print(json.load(sys.stdin).get("tool_input", {}).get("command", ""))' 2>/dev/null || true)"

# Only guard real pushes — ignore dry-runs and every non-push command.
case "$cmd" in
*"git push"*) ;;
*) exit 0 ;;
esac
if printf '%s' "$cmd" | grep -Eq -- '--dry-run|(^|[[:space:]])-n([[:space:]]|$)'; then
  exit 0
fi

root="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"

# Run a deterministic gate for real; on failure show the tail and block the push.
gate() {
  local label="$1"
  shift
  local out
  if ! out="$("$@" 2>&1)"; then
    echo "Pre-push gate: BLOCKED by $label — fix it and re-push." >&2
    printf '%s\n' "$out" | tail -20 >&2
    exit 2
  fi
}

# Cheapest first, so a stray marker fails fast before the ~20s of vet/tests.

# craft-residue: no CRAFT-FIX/CRAFT-DISPUTE marker may reach a push.
gate "craft-residue" go run -C "$root/cli/craft" . residue --root "$root"

# The craft module's own tests.
gate "craft tests" go test -C "$root/cli/craft" ./...

# deterministic-gates: craftsmanship-doc presence, gofmt, and go vet repo-wide.
gate "deterministic gates (fmt/vet/craft-doc)" make -C "$root" check-craft-doc fmt-check vet

exit 0
