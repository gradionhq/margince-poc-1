#!/usr/bin/env bash
#
# verify-boot.sh — the scripted half of gate D-H0 (human-boot-check).
#
# PRECONDITION (choice documented here, not started by this script): infra, migrations,
# seed data, and the API server must already be running — i.e. the caller has already
# run `make infra-up && make migrate-up && make seed-dev && make run` (or `make dev`).
# This script does NOT start `bin/server` itself. Reason: the server blocks in
# ListenAndServe with no built-in daemonize/health-check handshake, so starting it here
# would mean re-implementing backgrounding + readiness polling for a process the caller
# (a human at a terminal, or CI) is far better positioned to own and tear down. Keeping
# this script a pure client (curl only) also matches how it doubles as a CI gate script:
# CI already needs its own "bring the stack up" step for other reasons (see
# .github/workflows/factory-g0.yml), so duplicating that logic here would drift.
#
# What this script proves:
#   1. The API server is reachable and /auth/login works with the seeded admin
#      credentials, returning a session cookie.
#   2. That session cookie authorizes a GET /people that returns all three seeded
#      people (Alice, Bob, Carol).
#   3. The frontend actually builds (make fe-build) — a real compile+bundle check, not
#      just "a dist/ directory exists somewhere" (which could be stale).
#
# Usage: run from the skeleton/ directory (or pass SKELETON_DIR):
#   cd skeleton && bash scripts/verify-boot.sh
#
# Exits non-zero on any failure, with a clear error message identifying which step
# failed — usable as a CI gate script (see .github/workflows/factory-g0.yml).

set -euo pipefail

SKELETON_DIR="${SKELETON_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
API_BASE="${API_BASE:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme}"
WORKSPACE_ID="${WORKSPACE_ID:-00000000-0000-0000-0000-000000000001}"

COOKIE_JAR="$(mktemp -t verify-boot-cookies.XXXXXX)"
trap 'rm -f "$COOKIE_JAR"' EXIT

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

echo "== verify-boot: step 1/3 — login as seeded admin =="
login_status="$(curl -sS -o "${COOKIE_JAR}.body" -w '%{http_code}' \
  -c "$COOKIE_JAR" \
  -X POST "$API_BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")"

if [ "$login_status" != "200" ]; then
  echo "  response body:" >&2
  cat "${COOKIE_JAR}.body" >&2
  fail "POST /auth/login returned HTTP $login_status (expected 200). Is the server running at $API_BASE with the dev seed applied? (make infra-up && make migrate-up && make seed-dev && make run)"
fi
grep -q crm_session "$COOKIE_JAR" || fail "login returned 200 but no crm_session cookie was set — check cookie jar: $COOKIE_JAR"
echo "  OK: logged in as $ADMIN_EMAIL, session cookie captured"

echo "== verify-boot: step 2/3 — GET /people, assert Alice/Bob/Carol present =="
people_status="$(curl -sS -o "${COOKIE_JAR}.people" -w '%{http_code}' \
  -b "$COOKIE_JAR" \
  -H "X-Workspace-ID: $WORKSPACE_ID" \
  "$API_BASE/people")"

if [ "$people_status" != "200" ]; then
  echo "  response body:" >&2
  cat "${COOKIE_JAR}.people" >&2
  fail "GET /people returned HTTP $people_status (expected 200)"
fi

people_json="$(cat "${COOKIE_JAR}.people")"
for name in "Alice Müller" "Bob Schmidt" "Carol Wagner"; do
  if ! printf '%s' "$people_json" | jq -e --arg n "$name" '.data[] | select(.full_name == $n)' >/dev/null 2>&1; then
    echo "  full /people response:" >&2
    printf '%s\n' "$people_json" >&2
    fail "seeded person '$name' not found in GET /people response — seed data missing or stale (make seed-dev)"
  fi
  echo "  OK: found '$name'"
done

echo "== verify-boot: step 3/3 — frontend build =="
if ! (cd "$SKELETON_DIR" && make fe-build); then
  fail "make fe-build failed — see output above"
fi
echo "  OK: make fe-build succeeded"

echo ""
echo "verify-boot: ALL CHECKS GREEN"
