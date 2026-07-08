#!/usr/bin/env bash
# Contract-first codegen. Source of truth is the in-repo backend/api/crm.yaml (3.1).
#   TS:  openapi-typescript reads 3.1 directly.
#   Go:  3.1 -> openapi-down-convert -> 3.0 -> contract-disambiguate -> oapi-codegen.
# Usage: gen-types.sh [write|check]   (default write)
set -euo pipefail
MODE="${1:-write}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

CRM_YAML="${CRM_YAML:-backend/api/crm.yaml}"
OAPI="$(command -v oapi-codegen || echo "$HOME/go/bin/oapi-codegen")"
GO_DST="backend/internal/contracts/types/crm_gen.go"
TS_DST="frontend/src/lib/api-client/generated/crm.d.ts"

# Graceful skip when tooling isn't installed, so `make check` stays green on a
# bare machine. Run `make tools` + `make fe-install` to activate the real gate.
if [ ! -d node_modules ] || [ ! -x "$OAPI" ]; then
  echo "skip gen-types ($MODE): run 'make tools' + 'make fe-install' to activate contract codegen"
  exit 0
fi
if [ ! -f "$CRM_YAML" ]; then
  echo "skip gen-types ($MODE): contract not found at: $CRM_YAML"
  exit 0
fi

mkdir -p .tmp/gen "$(dirname "$GO_DST")" "$(dirname "$TS_DST")"
# Fast pre-flight: resolve every local $ref and fail with a precise message
# before the heavy redoc bundler aborts with a cryptic "Can't resolve $ref".
node scripts/contract-lint.mjs "$CRM_YAML"
pnpm exec openapi-typescript "$CRM_YAML" -o .tmp/gen/crm.d.ts >/dev/null
pnpm exec openapi-down-convert --input "$CRM_YAML" --output .tmp/gen/crm.3.0.yaml >/dev/null
node scripts/contract-disambiguate.mjs .tmp/gen/crm.3.0.yaml .tmp/gen/crm.3.0.fixed.yaml >/dev/null
"$OAPI" -config backend/api/oapi-types.cfg.yaml -o .tmp/gen/crm_gen.go .tmp/gen/crm.3.0.fixed.yaml >/dev/null
gofmt -w .tmp/gen/crm_gen.go
# oapi-codegen's chi-server output leaves stray blank lines gofmt doesn't
# touch (e.g. right after a wrapper func's opening brace); gofumpt -l is the
# fmt-check gate (§3.1), so run it here too when available.
GOFUMPT="$(command -v gofumpt || echo "$HOME/go/bin/gofumpt")"
[ -x "$GOFUMPT" ] && "$GOFUMPT" -w .tmp/gen/crm_gen.go

if [ "$MODE" = "check" ]; then
  fail=0
  diff -q "$GO_DST" .tmp/gen/crm_gen.go >/dev/null 2>&1 || { echo "DRIFT: $GO_DST is stale"; fail=1; }
  diff -q "$TS_DST" .tmp/gen/crm.d.ts  >/dev/null 2>&1 || { echo "DRIFT: $TS_DST is stale"; fail=1; }
  if [ "$fail" -ne 0 ]; then echo "contract drift — run 'make gen-types' and commit"; exit 1; fi
  echo "gen-types-check: generated types are up to date"
else
  cp .tmp/gen/crm_gen.go "$GO_DST"
  cp .tmp/gen/crm.d.ts  "$TS_DST"
  echo "gen-types: wrote $GO_DST + $TS_DST"
fi
