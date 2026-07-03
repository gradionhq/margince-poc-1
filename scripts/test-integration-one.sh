#!/usr/bin/env bash
# Run ONE integration package (optionally a single test) on a throwaway clone DB +
# private Redis db + private MinIO bucket — the fast inner-loop shortcut for agents
# and humans iterating on one test, without booting the whole parallel lane.
#
#   scripts/test-integration-one.sh DIR [RUN]
#     DIR  repo-root package dir, e.g. backend/internal/modules/directory or backend/cmd/api
#     RUN  optional -run regex, e.g. TestRelationshipEmploymentShape
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
# shellcheck source=scripts/lib-testdb.sh
source "$ROOT/scripts/lib-testdb.sh"

DIR="${1:-}"
RUN="${2:-}"
if [ -z "$DIR" ]; then
  echo "usage: $0 DIR [RUN]   (DIR e.g. backend/internal/modules/directory; RUN e.g. TestFoo)" >&2
  exit 2
fi

mr="$(module_for "$DIR")" || { echo "FAIL: '$DIR' is in no known go.work module" >&2; exit 2; }
mod="${mr%%|*}"; rel="${mr#*|}"

parse_test_dsn
db="margince_test_one_$$"
make_clone "$db"
trap 'drop_clone "$db"' EXIT

run_flag=()
[ -n "$RUN" ] && run_flag=(-run "$RUN")
echo "test-integration-one: $mod $rel ${RUN:+(-run $RUN) }(db=$db)"

( cd "$mod" \
    && TEST_DATABASE_URL="$(clone_dsn "$db")" \
       REDIS_URL="$(redis_url_for 0)" \
       BLOBSTORE_BUCKET="$(bucket_for one)" \
    go test -p 1 -tags=integration -v -count=1 -timeout=300s "${run_flag[@]+"${run_flag[@]}"}" "$rel" )
