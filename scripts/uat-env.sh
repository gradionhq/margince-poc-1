#!/usr/bin/env bash
# Per-worktree UAT env on the ONE shared infra (B5). Each slug gets its own
# database crm_uat_<slug> plus deterministic app/FE ports derived from the slug,
# so two worktrees can run a live UAT stack concurrently without colliding on
# the database or the host ports. All logs go to a single file; a state file
# records the PIDs + ports so `stop` can tear it down (killing by port too,
# since `go run` spawns a grandchild that outlives its parent).
#
#   scripts/uat-env.sh up   <slug>            # spin infra + db + backend + FE
#   scripts/uat-env.sh stop <slug> [--drop]   # stop servers; --drop also drops the db
set -euo pipefail

cmd="${1:-}"
slug="${2:-}"
shift 2 2>/dev/null || true
drop=0
for a in "$@"; do [ "$a" = "--drop" ] && drop=1; done

if [ -z "$slug" ]; then
  echo "FAIL: uat_env requires UAT_SLUG=<slug>" >&2
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

# Deterministic derivation (same slug → same db + ports, so a resume reuses the
# existing migrated+seeded data and rebinds the same host ports).
hash=$(printf '%s' "$slug" | cksum | awk '{print $1 % 1000}')
port=$(( 8080 + hash ))
fe_port=$(( 5173 + hash ))
db="crm_uat_${slug}"
uat_url="postgres://margince:margince@localhost:5432/${db}?sslmode=disable"
rundir=".tmp/uat/${slug}"
log="${rundir}/uat.log"
state="${rundir}/env"

migrate_bin=$(command -v migrate || true)
[ -z "$migrate_bin" ] && migrate_bin="$HOME/go/bin/migrate"

wait_ready() { # url timeout_s — any HTTP response (even 401) means the port is serving.
  for _ in $(seq 1 "$2"); do
    code=$(curl -s -o /dev/null -w "%{http_code}" "$1" 2>/dev/null || true)
    if [ -n "$code" ] && [ "$code" != "000" ]; then return 0; fi
    sleep 1
  done
  return 1
}

case "$cmd" in
up)
  mkdir -p "$rundir"
  : > "$log"
  echo "uat_env '$slug' → db=$db backend=:$port fe=:$fe_port (logs: $log)"
  {
    echo "=== infra + db ==="
    make infra-up
    make db-wait
    PGPASSWORD=margince psql -h localhost -U margince -d postgres \
      -c "CREATE DATABASE ${db}" 2>&1 || true
    "$migrate_bin" -path backend/migrations -database "$uat_url" up
    PGPASSWORD=margince psql -h localhost -U margince -d "$db" \
      -v ON_ERROR_STOP=1 -f backend/seed/reset.sql
    PGPASSWORD=margince psql -h localhost -U margince -d "$db" \
      -f backend/seed/dev.sql
    echo "=== build server (once, before the readiness poll) ==="
    make build
    echo "=== servers ==="
  } >>"$log" 2>&1

  # Run the compiled binary directly (not `go run`): starts in <1s so the poll
  # window is real, and $be_pid is the actual server process, so `stop` can kill
  # it cleanly (go run would leave an orphaned grandchild).
  ADDR=":${port}" DATABASE_URL="$uat_url" REDIS_URL="redis://localhost:6379" \
    BLOBSTORE_ENDPOINT="localhost:9000" BLOBSTORE_BUCKET="transcripts" \
    BLOBSTORE_ACCESS_KEY="minioadmin" BLOBSTORE_SECRET_KEY="minioadmin" \
    ./bin/server >>"$log" 2>&1 &
  be_pid=$!
  BACKEND_PORT="$port" pnpm --filter @gradion/crm-web exec vite --port "$fe_port" \
    >>"$log" 2>&1 &
  fe_pid=$!

  printf 'SLUG=%s\nPORT=%s\nFE_PORT=%s\nDB=%s\nBACKEND_PID=%s\nFE_PID=%s\nLOG=%s\n' \
    "$slug" "$port" "$fe_port" "$db" "$be_pid" "$fe_pid" "$log" >"$state"

  if wait_ready "http://localhost:${port}/people" 90 && wait_ready "http://localhost:${fe_port}/" 90; then
    echo "UAT env '$slug' ready"
    echo "  backend  http://localhost:${port}"
    echo "  frontend http://localhost:${fe_port}"
    echo "  logs     ${log}"
    echo "  stop     make uat_env_stop UAT_SLUG=${slug}"
  else
    echo "FAIL: uat_env '$slug' did not become ready in time — see ${log}" >&2
    exit 1
  fi
  ;;

stop)
  if [ -f "$state" ]; then
    # shellcheck disable=SC1090
    . "$state"
    kill "${BACKEND_PID:-}" "${FE_PID:-}" 2>/dev/null || true
    # go run spawns a compiled grandchild; free the ports by listener too.
    for p in "${PORT:-}" "${FE_PORT:-}"; do
      [ -n "$p" ] && lsof -ti "tcp:${p}" 2>/dev/null | xargs -r kill 2>/dev/null || true
    done
    rm -rf "$rundir"
    echo "stopped uat_env '$slug' (freed :${PORT:-?} :${FE_PORT:-?})"
  else
    # No state file — best-effort free the derived ports anyway.
    for p in "$port" "$fe_port"; do
      lsof -ti "tcp:${p}" 2>/dev/null | xargs -r kill 2>/dev/null || true
    done
    echo "no recorded env for '$slug' (freed derived ports :$port :$fe_port if bound)"
  fi
  if [ "$drop" = "1" ]; then
    PGPASSWORD=margince psql -h localhost -U margince -d postgres \
      -c "DROP DATABASE IF EXISTS ${db}" >/dev/null 2>&1 || true
    echo "dropped ${db}"
  fi
  ;;

*)
  echo "usage: uat-env.sh {up|stop} <slug> [--drop]" >&2
  exit 2
  ;;
esac
