#!/usr/bin/env bash
# Lint rule: enforce design-system discipline in production code.
#
# Fails on patterns that should never appear in NEW Margince web code:
#   1.  Hex literals (3/6/8-char) in *.tsx/.ts/.css (use var(--gf-*) instead)
#   1b. Raw color functions rgb()/rgba()/hsl()/oklch() (use a token / color-mix)
#   2.  Raw hardcoded text-[Npx] (use text-gf-{display|heading|body|caption|...})
#   3.  Forge-only semantic names without gf- prefix (page/content/modal/fast/p-md/etc.)
#   4.  Raw duration-{150|200|300|500} (use duration-gf-{fast|base|slow|slower})
#   5.  Raw z-{10|20|50} global layers (use z-gf-{dropdown|modal|overlay|toast|tooltip})
#   6.  Unmapped Tailwind palettes (emerald/rose/slate/sky/cyan/lime/fuchsia/purple/indigo)
#
# NOTE (post forge v0.7.0): Tailwind-default token names are ALLOWED without
# gf- prefix — bg-red-500, rounded-md, font-mono, max-w-2xl etc. resolve to
# forge brand-anchored values via @theme bridge. Only forge-only semantic names
# (no Tailwind equivalent) keep the gf- prefix.
#
# Scans *.tsx / *.ts / *.css under frontend/src and frontend/.storybook. Excludes test
# files and story files (.storybook config is in scope — it's hand-written and
# outside any other DS gate, so token discipline still applies there).
#
# Usage:
#   scripts/check-ds-purity.sh   # check all production files
#   scripts/check-ds-purity.sh <file1> <file2> ...  # lint-staged: check specific files
#
# Wired into `make ds-purity` / `make check-fe`.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Hand-written FE surface — production code + Storybook config. Excludes tests and stories.
TARGETS=(frontend/src frontend/.storybook)

# Allowed hex literals (third-party brand colors — document why here)
HEX_WHITELIST_RE='#(4285F4)\b'

# Build the file list.
FILES=()
if [[ "$#" -gt 0 ]]; then
  for f in "$@"; do
    case "$f" in
      frontend/src/*|*/frontend/src/*|frontend/.storybook/*|*/frontend/.storybook/*) ;;
      *) continue ;;
    esac
    case "$f" in *.test.*|*.stories.*) continue ;; esac
    # ledger-green.css is the brand-source file — raw hex is allowed there
    case "$f" in */styles/ledger-green.css) continue ;; esac
    case "$f" in *.tsx|*.ts|*.css) FILES+=("$f") ;; esac
  done
  if [[ "${#FILES[@]}" -eq 0 ]]; then
    echo "==> DS purity: no production FE files in scope — skipping"
    exit 0
  fi
else
  while IFS= read -r -d '' f; do FILES+=("$f"); done < <(
    find "${TARGETS[@]}" -type f \( -name "*.tsx" -o -name "*.ts" -o -name "*.css" \) \
      -not -name "*.test.*" -not -name "*.stories.*" \
      -not -name "ledger-green.css" \
      -print0 2>/dev/null
  )
fi

EXIT=0

check() {
  local label="$1"
  local pattern="$2"
  local hits
  hits=$(printf '%s\0' "${FILES[@]}" \
    | xargs -0 grep -nHE "$pattern" 2>/dev/null \
    | grep -vE "$HEX_WHITELIST_RE" \
    | grep -vE "var\(--gf-" \
    | grep -vE '&#[0-9a-fA-F]' \
    || true)
  if [[ -n "$hits" ]]; then
    echo ""
    echo "FAIL: $label"
    echo "$hits"
    EXIT=1
  fi
}

echo "==> DS purity check on production code (${#FILES[@]} files)"

# 1. Hex literals
check "Hex literals (use var(--gf-*) or Tailwind class instead)" \
  '#([0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b'

# 1b. Raw color functions
check "Raw color function (use var(--gf-*) or color-mix with a token)" \
  '(rgba?|hsla?|oklch)\('

# 2. Hardcoded text-[Npx] with known token equivalents
check "Hardcoded text-[Npx] (use text-gf-{caption|body|heading|display|...})" \
  '\btext-\[(10|11|12|13|14|15|18|22)px\]'

# 3. Forge-only semantic utilities without gf- prefix
check "Forge semantic without gf- prefix (e.g. bg-page → bg-gf-page)" \
  '\b(bg|text|border|ring|from|to|via|fill|stroke|outline|divide|placeholder|accent|caret|decoration|shadow)-(page|elevated|card|hover|accent|primary|content|secondary|tertiary|muted|subtle|strong|display|heading|subheading|body|caption|small|micro|label|status-success|status-warning|status-danger|status-info|status-success-fg|status-warning-fg|status-danger-fg|status-info-fg|status-success-subtle|status-warning-subtle|status-danger-subtle|status-info-subtle)([^a-zA-Z0-9_-]|$)'

check "Forge spacing without gf- prefix (e.g. p-md → p-gf-md)" \
  '\b(p|px|py|pt|pb|pl|pr|m|mx|my|mt|mb|ml|mr|gap|gap-x|gap-y|space-x|space-y)-(xs|sm|md|lg|xl|2xl)\b'

check "Forge duration without gf- prefix (e.g. duration-base → duration-gf-base)" \
  '\bduration-(instant|fast|base|slow|slower|glacial)\b'

check "Forge z-index without gf- prefix (e.g. z-modal → z-gf-modal)" \
  '\bz-(base|sticky|dropdown|overlay|modal|toast|tooltip|max)\b'

check "Forge ease-spring without gf- prefix (use ease-gf-spring)" \
  '\bease-spring\b'

# 4. Numeric duration
check "Numeric duration-N (use duration-gf-{instant|fast|base|slow|slower|glacial})" \
  '\bduration-(75|100|150|200|300|500|700|1000)\b'

# 5. Numeric z at global layer tiers
check "Numeric z-50+ global layer (use z-gf-{modal|toast|tooltip|max})" \
  '\bz-(50|60|70|80|90|100|999|9999)\b'

# 6. Unmapped Tailwind palettes
check "Unmapped Tailwind palette (use mapped equivalent: emerald→green, rose→pink, slate→neutral)" \
  '\b(bg|text|border|from|to|via|outline|ring|fill|stroke|caret|decoration|divide|placeholder|accent|shadow)-(emerald|slate|sky|cyan|lime|fuchsia|purple|indigo|rose)-(50|100|200|300|400|500|600|700|800|900|950)\b'

if [[ "$EXIT" == "0" ]]; then
  echo ""
  echo "PASS — DS purity OK on production code"
else
  echo ""
  echo "Fix the violations above. See .ai/skills/use-forge/SKILL.md for the token catalog."
  echo "If a hex is intentionally a third-party brand color, add it to HEX_WHITELIST_RE"
  echo "in this script and document why."
fi

exit $EXIT
