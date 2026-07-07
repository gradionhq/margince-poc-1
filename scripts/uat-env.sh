#!/usr/bin/env bash
# Per-worktree UAT env on the ONE shared infra (B5). Each slug gets its own
# database crm_uat_<slug> plus deterministic app/FE ports derived from the slug,
# so two worktrees can run a live UAT stack concurrently without colliding on
# the database or the host ports. All logs go to a single file; a state file
# records the PIDs + ports so `stop` can tear it down. The backend runs as a
# direct ./bin/server (killable by PID); the FE runs via pnpm (whose vite child
# outlives a bare `kill $pnpm_pid`), so `stop` also frees the recorded ports by
# listener as a backstop.
#
# Credentials are NOT hardcoded here: the connection URLs are derived from
# $DATABASE_URL and the Redis/blobstore endpoints are inherited from the
# environment — both exported by the Makefile (the single source, from
# docker-compose.dev.yml), so this script carries no secret literal.
#
#   scripts/uat-env.sh up   <slug>            # spin infra + db + backend + FE
#   scripts/uat-env.sh stop <slug> [--drop]   # stop servers; --drop also drops the db
set -euo pipefail

cmd="${1:-}"
slug="${2:-}"
shift 2 2>/dev/null || true
drop=0
for a in "$@"; do [[ "$a" == "--drop" ]] && drop=1; done

if [[ -z "$slug" ]]; then
  echo "FAIL: uat_env requires UAT_SLUG=<slug>" >&2
  exit 1
fi
# The slug flows into a filesystem path and a CREATE DATABASE identifier — keep it
# to a safe charset so it can neither traverse paths nor break/inject SQL.
if ! [[ "$slug" =~ ^[a-z0-9_-]+$ ]]; then
  echo "FAIL: UAT_SLUG must match ^[a-z0-9_-]+$ (got '$slug')" >&2
  exit 1
fi

: "${DATABASE_URL:?uat_env expects DATABASE_URL in the environment (run via 'make uat_env')}"
cd "$(git rev-parse --show-toplevel)"

# Deterministic derivation (same slug → same db + ports, so a resume reuses the
# existing migrated+seeded data and rebinds the same host ports).
hash=$(printf '%s' "$slug" | cksum | awk '{print $1 % 1000}')
port=$(( 8080 + hash ))
fe_port=$(( 5173 + hash ))
db="crm_uat_${slug}"

# Build the per-slug + admin connection URLs by swapping the database segment of
# the exported base DATABASE_URL — no credential literal lives in this script.
db_query="sslmode=disable"
[[ "$DATABASE_URL" == *\?* ]] && db_query="${DATABASE_URL#*\?}"
conn_prefix="${DATABASE_URL%%\?*}"   # strip query
conn_prefix="${conn_prefix%/*}"      # strip the base db name → scheme://user:pass@host:port
uat_url="${conn_prefix}/${db}?${db_query}"
admin_url="${conn_prefix}/postgres?${db_query}"

rundir=".tmp/uat/${slug}"
log="${rundir}/uat.log"
state="${rundir}/env"

migrate_bin=$(command -v migrate || true)
[[ -z "$migrate_bin" ]] && migrate_bin="$HOME/go/bin/migrate"

wait_ready() { # url timeout_s — any HTTP response (even 401) means the port is serving.
  for _ in $(seq 1 "$2"); do
    code=$(curl -s -o /dev/null -w "%{http_code}" "$1" 2>/dev/null || true)
    [[ -n "$code" && "$code" != "000" ]] && return 0
    sleep 1
  done
  return 1
}

case "$cmd" in
up)
  # Refuse if the derived port is already bound — otherwise a second `up` for a
  # still-running slug would fail to bind silently and wait_ready would get a
  # false "ready" from the OLD server. Stop it first.
  if lsof -ti "tcp:${port}" >/dev/null 2>&1; then
    echo "FAIL: backend port :${port} already in use — is env '$slug' already running? (make uat_env_stop UAT_SLUG=$slug)" >&2
    exit 1
  fi
  mkdir -p "$rundir"
  : > "$log"
  echo "uat_env '$slug' → db=$db backend=:$port fe=:$fe_port (logs: $log)"
  {
    echo "=== infra + db ==="
    make infra-up
    make db-wait
    psql "$admin_url" -c "CREATE DATABASE ${db}" 2>&1 || true
    "$migrate_bin" -path backend/migrations -database "$uat_url" up
    psql "$uat_url" -v ON_ERROR_STOP=1 -f backend/seed/reset.sql
    psql "$uat_url" -v ON_ERROR_STOP=1 -f backend/seed/dev.sql
    echo "=== build server (once, before the readiness poll) ==="
    make build
    echo "=== servers ==="
  } >>"$log" 2>&1

  # Run the compiled binary directly (not `go run`): starts in <1s so the poll
  # window is real, and $be_pid is the actual server process, so `stop` can kill
  # it cleanly (go run would leave an orphaned grandchild). Redis + blobstore env
  # are inherited from the Makefile (exported); only ADDR + DATABASE_URL differ.
  ADDR=":${port}" DATABASE_URL="$uat_url" ./bin/server >>"$log" 2>&1 &
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
  if [[ -f "$state" ]]; then
    # shellcheck disable=SC1090
    . "$state"
    kill "${BACKEND_PID:-}" "${FE_PID:-}" 2>/dev/null || true
    # Backstop: free the recorded ports by listener (reaps vite, pnpm's child).
    for p in "${PORT:-}" "${FE_PORT:-}"; do
      [[ -n "$p" ]] || continue
      pids=$(lsof -ti "tcp:${p}" 2>/dev/null || true)
      [[ -n "$pids" ]] && kill $pids 2>/dev/null || true
    done
    rm -rf "$rundir"
    echo "stopped uat_env '$slug' (freed :${PORT:-?} :${FE_PORT:-?})"
  else
    # No state file — best-effort free the derived ports anyway.
    for p in "$port" "$fe_port"; do
      pids=$(lsof -ti "tcp:${p}" 2>/dev/null || true)
      [[ -n "$pids" ]] && kill $pids 2>/dev/null || true
    done
    echo "no recorded env for '$slug' (freed derived ports :$port :$fe_port if bound)"
  fi
  if [[ "$drop" == "1" ]]; then
    psql "$admin_url" -c "DROP DATABASE IF EXISTS ${db}" >/dev/null 2>&1 || true
    echo "dropped ${db}"
  fi
  ;;

*)
  echo "usage: uat-env.sh {up|stop} <slug> [--drop]" >&2
  exit 2
  ;;
esac
