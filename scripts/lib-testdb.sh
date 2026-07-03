#!/usr/bin/env bash
# Shared helpers for the integration test lanes (test-integration-parallel.sh +
# test-integration-one.sh): parse the template DSN, clone/drop throwaway DBs, and
# derive a per-slot Redis db-index + MinIO bucket. Source this; don't execute it.

# parse_test_dsn: reads TEST_DATABASE_URL (or the Makefile default) and exports the
# psql-admin pieces + the template db name + the query string.
parse_test_dsn() {
  local url="${TEST_DATABASE_URL:-postgres://margince:margince@localhost:5432/margince_test?sslmode=disable}"
  local proto_stripped="${url#*://}"
  local creds_host="${proto_stripped%%/*}"          # margince:margince@localhost:5432
  local userpass="${creds_host%@*}" hostport="${creds_host#*@}"
  PGUSER_="${userpass%%:*}"; PGPASS_="${userpass#*:}"
  PGHOST_="${hostport%%:*}"; PGPORT_="${hostport#*:}"; [ "$PGPORT_" = "$hostport" ] && PGPORT_=5432
  local dbpart="${proto_stripped#*/}"               # margince_test?sslmode=disable
  TEMPLATE_DB="${dbpart%%\?*}"
  QUERY="${url#*\?}"; [ "$QUERY" = "$url" ] && QUERY=""
  export PGUSER_ PGPASS_ PGHOST_ PGPORT_ TEMPLATE_DB QUERY
}

pg_admin() { PGPASSWORD="$PGPASS_" psql -h "$PGHOST_" -p "$PGPORT_" -U "$PGUSER_" -d postgres "$@"; }

clone_dsn() { local db="$1"; echo "postgres://${PGUSER_}:${PGPASS_}@${PGHOST_}:${PGPORT_}/${db}${QUERY:+?$QUERY}"; }

make_clone() { # db — drop any stale clone, then clone the migrated template (a fast file copy)
  pg_admin -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS $1" -c "CREATE DATABASE $1 TEMPLATE $TEMPLATE_DB" >/dev/null
}

drop_clone() { pg_admin -c "DROP DATABASE IF EXISTS $1" >/dev/null 2>&1 || true; }

# redis_url_for SLOT [BASE] — normalize BASE to host:port, then pin logical db = SLOT % 16.
redis_url_for() {
  local slot="$1" base="${2:-${REDIS_URL:-redis://localhost:6379}}"
  local root="${base%/}"
  [[ "$root" =~ /[0-9]+$ ]] && root="${root%/*}"
  echo "${root}/$(( slot % 16 ))"
}

# bucket_for SLOT [BASE] — DNS-compliant bucket name (hyphen, not underscore).
bucket_for() { echo "${2:-${BLOBSTORE_BUCKET:-transcripts}}-p${1}"; }

# module_for DIR — map a repo-root package dir to "MODULE_DIR|REL_PKG" (longest-prefix
# match against the go.work module roots). Returns 1 if DIR is in no known module.
module_for() {
  local dir="${1%/}" m best="" rel
  for m in backend crm-de cli/crm-gen cli/craft; do
    case "$dir/" in "$m"/*) [ ${#m} -gt ${#best} ] && best="$m" ;; esac
  done
  [ -n "$best" ] || return 1
  rel="./${dir#"$best"/}"; [ "$dir" = "$best" ] && rel="."
  echo "$best|$rel"
}
