#!/usr/bin/env bash
# Lint rule: UI chrome icons are Lucide only — never emoji/Unicode glyphs.
# Sanctioned exceptions: the 🟢/🟡 autonomy-tier dots inside the `.dot` component
# (frontend/src/ui/AutonomyDot.tsx), bound to --gf-online/--gf-away; and generated
# contract types (lib/api-client/generated/) whose doc comments quote crm.yaml
# prose verbatim — never hand-edited, not rendered UI chrome.
# Mirrors frontend/scripts/check-font-lock.sh. Wired into make icon-lint / check-fe.
#
# Usage:
#   frontend/scripts/check-icon-glyph.sh [<dir>]
#     <dir> defaults to frontend/src next to this script's parent.
#     Pass a temp directory for fixture testing.
#
# Uses perl -CSD for Unicode scanning (BSD grep lacks -P / PCRE).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCAN_DIR="${1:-"$(cd "$SCRIPT_DIR/.." && pwd)/src"}"

# Collect files (bash 3 compatible — no mapfile)
FILES=()
while IFS= read -r -d '' f; do FILES+=("$f"); done < <(
  find "$SCAN_DIR" -type f \( -name "*.ts" -o -name "*.tsx" -o -name "*.css" \) \
    -not -name "*.test.*" -not -name "*.stories.*" \
    -not -name "AutonomyDot.tsx" \
    -not -path "*/lib/api-client/generated/*" \
    -print0 2>/dev/null
)

if [[ "${#FILES[@]}" -eq 0 ]]; then
  echo "==> Icon-glyph: no files to scan in $SCAN_DIR"
  exit 0
fi

echo "==> Icon-glyph check (${#FILES[@]} files in $SCAN_DIR)"

EXIT=0

# Scan for emoji/pictographic Unicode ranges using perl (BSD grep lacks -P).
# Ranges covered:
#   U+1F300-U+1FAFF  Misc Symbols & Pictographs, Emoticons, Transport, Supplemental Symbols
#   U+1F1E6-U+1F1FF  Regional indicator symbols (flag sequences)
#   U+2600-U+27BF    Misc Symbols, Dingbats
#   U+2B00-U+2BFF    Misc Symbols and Arrows
#   U+FE0F           Variation selector-16 (emoji presentation selector)
while IFS= read -r hit; do
  [[ -z "$hit" ]] && continue
  echo "FAIL (emoji/glyph used as UI chrome — use a Lucide Icon): $hit"
  EXIT=1
done < <(
  printf '%s\0' "${FILES[@]}" \
    | xargs -0 perl -CSD -ne '
      if (/[\x{1F300}-\x{1FAFF}\x{1F1E6}-\x{1F1FF}\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{FE0F}]/) {
        chomp; print $ARGV . ":" . $. . ":" . $_ . "\n"
      }
      close ARGV if eof
    ' 2>/dev/null \
  || true
)

echo ""
if [[ "$EXIT" == "0" ]]; then
  echo "PASS — Icon-glyph OK (Lucide-only; 🟢/🟡 dots confined to AutonomyDot.tsx)"
else
  echo "Fix: replace the emoji with a Lucide <Icon name=…/> or the AutonomyDot component."
fi
exit $EXIT
