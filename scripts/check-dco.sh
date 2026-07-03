#!/usr/bin/env bash
# DCO gate: every commit in the PR range must carry a Signed-off-by trailer
# (git commit -s). Fails listing the offenders. A lightweight stand-in for the
# DCO GitHub App so the check has no external dependency. See CONTRIBUTING.md.
set -euo pipefail

base="${1:-origin/main}"
head="${2:-HEAD}"

missing=0
# --no-merges: skip merge commits — notably the ephemeral merge commit GitHub
# synthesizes for pull_request runs, which has no sign-off and never can.
while IFS= read -r sha; do
  [ -z "$sha" ] && continue
  if ! git log -1 --format=%B "$sha" | grep -qiE '^Signed-off-by: .+ <.+@.+>'; then
    echo "missing Signed-off-by: $(git log -1 --format='%h %s' "$sha")"
    missing=$((missing + 1))
  fi
done < <(git rev-list --no-merges "${base}..${head}")

if [ "$missing" -gt 0 ]; then
  echo "DCO: $missing commit(s) without sign-off — amend with 'git commit -s' (see CONTRIBUTING.md)"
  exit 1
fi
echo "DCO: all commits signed off"
