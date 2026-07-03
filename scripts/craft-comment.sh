#!/usr/bin/env bash
# Post the craftsmanship reviewer's MAJOR/MINOR findings as non-blocking inline PR
# comments. BLOCKER findings are NOT posted here — they become CRAFT-FIX markers in
# source (B-EP11.4) and gate the merge via the verdict step. Reads the canonical
# result JSON; idempotency is best-effort (a re-run re-comments — acceptable for an
# advisory channel).
set -euo pipefail

result="${1:?usage: craft-comment.sh <result.json> <pr_number>}"
pr="${2:?usage: craft-comment.sh <result.json> <pr_number>}"
repo="${GITHUB_REPOSITORY:-gradionhq/margince-poc}"

# `.findings // []` tolerates a null/absent findings list (e.g. a skipped review).
count=$(jq '[(.findings // [])[] | select(.severity=="MAJOR" or .severity=="MINOR")] | length' "$result")
if [ "$count" -eq 0 ]; then
  echo "no MAJOR/MINOR findings to comment"
  exit 0
fi

jq -c '(.findings // [])[] | select(.severity=="MAJOR" or .severity=="MINOR")' "$result" | while read -r f; do
  file=$(jq -r '.file' <<<"$f")
  line=$(jq -r '.line' <<<"$f")
  sev=$(jq -r '.severity' <<<"$f")
  cat=$(jq -r '.category' <<<"$f")
  why=$(jq -r '.rationale' <<<"$f")
  fix=$(jq -r '.suggested_fix' <<<"$f")
  body="**craftsmanship · ${sev} · ${cat}** (non-blocking)"$'\n\n'"${why}"$'\n\n'"_Suggested fix:_ ${fix}"
  gh api "repos/${repo}/pulls/${pr}/comments" \
    -f body="$body" -f commit_id="$(git rev-parse HEAD)" -f path="$file" -F line="$line" -f side=RIGHT \
    >/dev/null 2>&1 || echo "warn: could not anchor comment on ${file}:${line} (line may be outside the diff)"
done
echo "posted ${count} non-blocking craftsmanship comment(s)"
