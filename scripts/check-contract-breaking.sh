#!/usr/bin/env bash
# Contract-drift gate, breaking-change half. Severity-classifies every change to
# backend/api/crm.yaml since the base ref and fails on ERR-level (breaking) changes;
# WARN/INFO-level changes (additive, deprecation) pass. See docs/quality/quality-gates.md.
# Usage: check-contract-breaking.sh [base-ref]   (default: origin/main)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

CRM_YAML="${CRM_YAML:-backend/api/crm.yaml}"
# Phase 1c moved the contract from contract/crm.yaml -> backend/api/crm.yaml.
# A base ref cut before that move still has the file at the old path — fall
# back to it so the gate keeps comparing the same file across the rename
# instead of silently no-op-skipping forever.
OLD_CRM_YAML="contract/crm.yaml"
BASE_REF="${1:-origin/main}"

# Graceful skip when tooling isn't installed, so `make check` stays green on a
# bare machine. Run `make tools` to activate the real gate.
if ! command -v oasdiff >/dev/null 2>&1; then
  echo "skip contract-breaking-check: run 'make tools' to install oasdiff"
  exit 0
fi
if [ ! -f "$CRM_YAML" ]; then
  echo "skip contract-breaking-check: contract not found at: $CRM_YAML"
  exit 0
fi
if ! git rev-parse --verify -q "$BASE_REF" >/dev/null; then
  echo "skip contract-breaking-check: base ref '$BASE_REF' not found (nothing to diff against)"
  exit 0
fi

BASE_PATH="$CRM_YAML"
if ! git cat-file -e "$BASE_REF:$CRM_YAML" 2>/dev/null; then
  if git cat-file -e "$BASE_REF:$OLD_CRM_YAML" 2>/dev/null; then
    BASE_PATH="$OLD_CRM_YAML"
    echo "contract-breaking-check: $CRM_YAML is new at $BASE_REF; diffing against pre-move path $OLD_CRM_YAML instead"
  else
    echo "skip contract-breaking-check: contract did not exist at $BASE_REF under either $CRM_YAML or $OLD_CRM_YAML (nothing to diff)"
    exit 0
  fi
fi

if ! oasdiff breaking "$BASE_REF:$BASE_PATH" "$CRM_YAML" --fail-on ERR -f text; then
  echo "contract-breaking-check: breaking API change(s) since $BASE_REF — fix or deprecate instead of removing" >&2
  exit 1
fi
echo "contract-breaking-check: no breaking API changes since $BASE_REF"
