#!/usr/bin/env bash
# Subsystem-doc style gate (AGENTS.md "Domain-doc style — NON-NEGOTIABLE").
# A subsystem chapter has two regions (see docs/subsystems/_TEMPLATE.md):
#   PROSE  — everything above the single `## Appendix` marker: a SYSTEM EXPLANATION,
#            never a code walkthrough. The ban-list below rejects the unambiguous tells.
#   APPENDIX — everything below the marker: pinned normative facts. Fences are legal
#            here; instead the appendix lint applies — fixed subsection vocabulary and
#            a `Source:` citation line opening every subsection.
# A doc with no marker is all prose. More than one marker is a failure.
# Underscore-prefixed docs (_TEMPLATE.md) are the authoring guide itself and are not scanned.
#
# Usage: check-subsystem-doc-style.sh [TARGET_DIR]
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-docs/subsystems}"
case "$TARGET" in
  /*) SCAN="$TARGET" ;;
  *)  SCAN="$REPO_ROOT/$TARGET" ;;
esac

# Prose rules: "label|regex". Tuned so module DIRECTORIES, doc cross-links (*.md),
# latency budgets (200 ms), and provenance ids (ADR-0026, P12) all pass.
RULES=(
  'code fence|```'
  'source filename|[A-Za-z0-9_/-]+\.(go|tsx|ts|sql)\b'
  'migration path|(infra/migrations|backend/migrations)'
  'migration number|(migration[s]?[[:space:]]+[0-9]{3,6}\b|\b0000[0-9][0-9]\b)'
  'PR/issue ref|(^|[^A-Za-z0-9])#[0-9]+\b'
  'function signature|`[A-Za-z_][A-Za-z0-9_]*\([^`]*\)`'
  'HTTP endpoint|\b(GET|POST|PUT|PATCH|DELETE)[[:space:]]+/'
)
APPENDIX_VOCAB='Parameters|Formulas|Schema|Wire|Events|Limits|Tools|Acceptance|Seed'

fail=0
report=""
while IFS= read -r -d '' doc; do
  base="$(basename "$doc")"
  case "$base" in _*) continue ;; esac

  markers="$(grep -c '^## Appendix$' "$doc" || true)"
  if [ "$markers" -gt 1 ]; then
    fail=1
    report+="  ${base} — appendix marker: ${markers} markers (exactly one allowed)"$'\n'
    continue
  fi

  if [ "$markers" -eq 1 ]; then
    marker_line="$(grep -n '^## Appendix$' "$doc" | cut -d: -f1)"
    prose="$(head -n "$((marker_line - 1))" "$doc")"
    appendix="$(tail -n "+$((marker_line + 1))" "$doc")"
  else
    prose="$(cat "$doc")"
    appendix=""
  fi

  for rule in "${RULES[@]}"; do
    label="${rule%%|*}"
    regex="${rule#*|}"
    hits="$(printf '%s\n' "$prose" | grep -nE "$regex" || true)"
    if [ -n "$hits" ]; then
      fail=1
      report+="  ${base} — prose ${label}:"$'\n'
      report+="$(printf '%s\n' "$hits" | sed 's/^/      /')"$'\n'
    fi
  done

  if [ -n "$appendix" ]; then
    badheads="$(printf '%s\n' "$appendix" | grep -nE '^### ' | grep -vE "^[0-9]+:### (${APPENDIX_VOCAB})\b" || true)"
    if [ -n "$badheads" ]; then
      fail=1
      report+="  ${base} — appendix subsection outside the fixed vocabulary:"$'\n'
      report+="$(printf '%s\n' "$badheads" | sed 's/^/      /')"$'\n'
    fi
    heads="$(printf '%s\n' "$appendix" | grep -cE '^### ' || true)"
    sources="$(printf '%s\n' "$appendix" | grep -cE '^Source:' || true)"
    if [ "$heads" -gt 0 ] && [ "$sources" -lt "$heads" ]; then
      fail=1
      report+="  ${base} — appendix citations: ${heads} subsections but only ${sources} 'Source:' lines"$'\n'
    fi
  fi
done < <(find "$SCAN" -maxdepth 1 -name '*.md' -type f -print0)

if [ "$fail" -ne 0 ]; then
  echo "FAIL: subsystem docs violate the two-region doc style (AGENTS.md 'Domain-doc style'):"
  printf '%s' "$report"
  echo "Prose explains, appendices pin — see docs/subsystems/_TEMPLATE.md."
  exit 1
fi
echo "ok: subsystem docs are system explanations with conforming pinned appendices"
