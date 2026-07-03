#!/usr/bin/env bash
# Parallel integration-test runner.
#
# The serial form runs every integration package with `go test -p 1` against ONE shared
# margince_test DB, because parallel packages racing on the same DB collide on shared rows
# + golang-migrate's advisory lock. That serialization is the only reason for `-p 1` — the
# tests themselves are I/O-bound (mostly idle, waiting on Postgres), so the CPU sits unused.
#
# This runner removes the shared-state constraint instead of the serialization. Each package
# gets three tiers of private throwaway state, so concurrent packages share nothing:
#   Postgres — a clone of the migrated template (CREATE DATABASE ... TEMPLATE, a fast file copy).
#   Redis    — a private logical db index (REDIS_URL .../<idx % 16>). Only 16 indices exist,
#              so >16 redis-using packages would wrap — harmless, they also key uniquely.
#   MinIO    — a private bucket (BLOBSTORE_BUCKET=<base>-p<idx>), auto-created by NewMinIOStore.
# Within a package nothing changes — still `-p 1`, the same sequential model that's green today
# — so no test file needs editing.
#
# Same teeth as the serial lane: zero-skip guard (a SKIP fails the run) and any package failure
# fails the whole run.
#
# Env:
#   INTEGRATION_JOBS   max concurrent packages (default: min(nproc, 8))
#   TEST_DATABASE_URL  template DB DSN (default from Makefile)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
# shellcheck source=scripts/lib-testdb.sh
source "$ROOT/scripts/lib-testdb.sh"

GO_DIRS=(backend crm-de cli/crm-gen cli/craft)
parse_test_dsn

JOBS="${INTEGRATION_JOBS:-$(( $(sysctl -n hw.ncpu 2>/dev/null || nproc) < 8 ? $(sysctl -n hw.ncpu 2>/dev/null || nproc) : 8 ))}"

# Build the work list: "module_dir|relative_pkg" for every package with an integration test.
# (Packages without a //go:build integration file add nothing here beyond `make test`.)
WORK="$(mktemp)"
for d in "${GO_DIRS[@]}"; do
  [ -d "$d" ] || continue
  # grep exits 1 on no-match; tolerate it under set -e/pipefail.
  matches="$(grep -rl "go:build integration" --include="*.go" "$d" 2>/dev/null || true)"
  [ -n "$matches" ] || continue
  printf '%s\n' "$matches" | xargs -n1 dirname | sort -u | while IFS= read -r pkgdir; do
    rel="./${pkgdir#"$d"/}"; [ "$pkgdir" = "$d" ] && rel="."
    echo "$d|$rel"
  done
done > "$WORK"

NPKGS=$(wc -l < "$WORK" | tr -d ' ')
echo "test-integration-parallel: $NPKGS packages, up to $JOBS concurrent, template=$TEMPLATE_DB"

OUTDIR="$(mktemp -d)"; trap 'rm -f "$WORK"; rm -rf "$OUTDIR"' EXIT

# One job = clone a throwaway DB + own a private Redis db and MinIO bucket, run that
# package against them, drop the clone.
run_one() {
  local line="$1" idx="$2" outdir="$3"
  local d="${line%%|*}" rel="${line#*|}"
  local db="margince_test_p${idx}_$$"
  local log="$outdir/$idx.log"
  local redis_url bucket; redis_url="$(redis_url_for "$idx")"; bucket="$(bucket_for "$idx")"
  {
    echo "=== integration $d $rel (db=$db redis=$redis_url bucket=$bucket) ==="
    make_clone "$db"
    local st=0
    ( cd "$d" \
        && TEST_DATABASE_URL="$(clone_dsn "$db")" REDIS_URL="$redis_url" BLOBSTORE_BUCKET="$bucket" \
        go test -p 1 -tags=integration -v -count=1 -timeout=300s "$rel" ) || st=$?
    drop_clone "$db"
    echo "EXIT $st"
  } > "$log" 2>&1
}
export -f run_one clone_dsn make_clone drop_clone pg_admin redis_url_for bucket_for

# Fan out with a bounded worker pool. nl numbers the lines → stable per-job DB names + logs.
nl -ba -w1 -s'|' "$WORK" \
  | xargs -P "$JOBS" -I{} bash -c 'line="{}"; idx="${line%%|*}"; rest="${line#*|}"; run_one "$rest" "$idx" "'"$OUTDIR"'"'

# Aggregate: print every log in package (idx) order, then enforce failure + zero-skip teeth.
fail=0
for base in $(cd "$OUTDIR" && ls -1 -- *.log | sort -n); do
  log="$OUTDIR/$base"
  cat "$log"
  grep -q "^EXIT 0$" "$log" || fail=1
done

if grep -rq -- '--- SKIP' "$OUTDIR"; then
  echo "FAIL: integration tests must not skip — provision the env/service, do not skip:"
  grep -rh -- '--- SKIP' "$OUTDIR"
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo "FAIL: integration tests failed (parallel, $NPKGS packages) — see package logs above"
  exit 1
fi
# Keep the exact success sentinel the swarm gates grep for; the count is informational.
echo "OK: integration passed with 0 skips ($NPKGS packages, parallel)"
