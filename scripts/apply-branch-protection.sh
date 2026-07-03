#!/usr/bin/env bash
# Apply infra/branch-protection.json to main. Run in Phase 4, after the golden set
# calibrates BLOCK precision (B-EP11.6) — this is the moment the craftsmanship check
# becomes merge-blocking with no override. Idempotent.
set -euo pipefail

repo="${1:-gradionhq/margince-poc}"
branch="${2:-main}"
cfg="$(dirname "$0")/../infra/branch-protection.json"

jq 'del(._comment)' "$cfg" | gh api -X PUT "repos/${repo}/branches/${branch}/protection" \
  -H "Accept: application/vnd.github+json" --input -
echo "applied branch protection to ${repo}@${branch}"
