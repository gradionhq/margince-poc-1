#!/usr/bin/env bash
# Lint rule: enforce font-family discipline in frontend/src.
#
# Fails if any font-family declaration uses a family that is NOT one of:
#   - Outfit  (display)
#   - DM Sans (body/heading/label)
#   - JetBrains Mono (mono)
#
# Generic fallbacks are always allowed: system-ui, sans-serif, ui-monospace,
# monospace.  Test files, story files, and the brand-source override file are
# excluded.
#
# Usage:
#   frontend/scripts/check-font-lock.sh [<dir>]
#     <dir> defaults to frontend/src next to this script's parent.
#     Pass a temp directory for fixture testing.
#
# Wired into `make font-lock` / `make check-fe`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCAN_DIR="${1:-"$(cd "$SCRIPT_DIR/.." && pwd)/src"}"

# Collect files (bash 3 compatible — no mapfile)
FILES=()
while IFS= read -r -d '' f; do FILES+=("$f"); done < <(
  find "$SCAN_DIR" -type f \( -name "*.ts" -o -name "*.tsx" -o -name "*.css" \) \
    -not -name "*.test.*" -not -name "*.stories.*" \
    -not -name "ledger-green.css" \
    -print0 2>/dev/null
)

if [[ "${#FILES[@]}" -eq 0 ]]; then
  echo "==> Font-lock: no files to scan in $SCAN_DIR"
  exit 0
fi

echo "==> Font-lock check (${#FILES[@]} files in $SCAN_DIR)"

EXIT=0

# For each font-family declaration line, strip allowed families and
# check if any disallowed family name remains.
while IFS= read -r hit; do
  # Extract just the value part after font-family:
  value=$(echo "$hit" | grep -oE "font-family\s*:[^;]+" | head -1)
  [[ -z "$value" ]] && continue

  # Remove allowed families and generic tokens (quotes, commas, spaces, colon)
  stripped=$(echo "$value" \
    | sed -E 's/font-family\s*://g' \
    | sed -E 's/Outfit//g' \
    | sed -E 's/DM Sans//g' \
    | sed -E 's/JetBrains Mono//g' \
    | sed -E 's/system-ui//g' \
    | sed -E 's/sans-serif//g' \
    | sed -E 's/ui-monospace//g' \
    | sed -E 's/monospace//g' \
    | tr -d '",'"'"',; \t')

  if [[ -n "$stripped" ]]; then
    echo "FAIL (disallowed font-family): $hit"
    EXIT=1
  fi
done < <(
  printf '%s\0' "${FILES[@]}" \
    | xargs -0 grep -nHE "font-family\s*:" 2>/dev/null \
  || true
)

echo ""
if [[ "$EXIT" == "0" ]]; then
  echo "PASS — Font-lock OK (only Outfit / DM Sans / JetBrains Mono)"
else
  echo "Fix the violations above. Allowed families: Outfit, DM Sans, JetBrains Mono."
  echo "Generic fallbacks (system-ui, sans-serif, ui-monospace, monospace) are OK."
fi

exit $EXIT
