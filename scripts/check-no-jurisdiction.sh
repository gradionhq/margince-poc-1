#!/usr/bin/env bash
# Fitness function (ADR-0042 / architecture/03): country-specific identifiers
# must live in a jurisdiction pack (crm-de), never in core. Fails the build if a
# country identifier appears in the scanned tree outside the Tier-0 `jurisdiction` seam.
#
# Usage: check-no-jurisdiction.sh [TARGET_DIR]
#   TARGET_DIR defaults to "backend" (resolved against the repo root) so the default
#   invocation — `make fitness-jurisdiction` — is unchanged. A test may pass an
#   absolute temp-dir path to exercise the gate against a fixture.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-backend}"
# Relative targets resolve against the repo root (preserves the default behaviour);
# absolute targets (e.g. a test's t.TempDir()) are scanned as-is.
case "$TARGET" in
  /*) SCAN="$TARGET" ;;
  *)  SCAN="$REPO_ROOT/$TARGET" ;;
esac

# Primary gate: named regulatory/country identifiers. Firm — these are unambiguous.
NAMED='XRechnung|ZUGFeRD|DATEV|GoBD|eIDAS|Impressum'
hits="$(grep -rniE "$NAMED" "$SCAN" --include='*.go' 2>/dev/null | grep -v '/jurisdiction/' || true)"

# Secondary gate (conservative ISO-3166): a quoted upper-case alpha-2 literal only
# when it appears on the same line as a country-ish keyword. This avoids
# false-positives on incidental two-letter strings (HTTP verbs, enum codes).
# Case-sensitive (no -i): the alpha-2 must be UPPER-case to count; the keyword
# alternation lists both cases explicitly so context still matches case-insensitively.
KW='[Cc]ountry|[Jj]urisdiction|[Ii][Ss][Oo][_-]?3166'
ISO="($KW).*\"[A-Z]{2}\"|\"[A-Z]{2}\".*($KW)"
iso_hits="$(grep -rnE "$ISO" "$SCAN" --include='*.go' 2>/dev/null | grep -v '/jurisdiction/' || true)"

if [ -n "$hits" ] || [ -n "$iso_hits" ]; then
  echo "FAIL: jurisdiction-specific strings found in core (move to crm-de):"
  [ -n "$hits" ] && echo "$hits"
  [ -n "$iso_hits" ] && echo "$iso_hits"
  exit 1
fi
echo "ok: no jurisdiction strings in core"
