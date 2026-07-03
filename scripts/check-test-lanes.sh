#!/usr/bin/env bash
# Rule 1 — test-lane separation.
#
# A unit test (one WITHOUT a `//go:build integration` or `//go:build liveuat` tag,
# so it runs under `make test`) must never open a REAL Postgres/Redis connection.
# Anything that needs real infra belongs in the integration lane (or the liveuat
# lane). This keeps `make test` hermetic and kills the "DB test that silently skips
# in the unit lane" anti-pattern.
#
# Fakes are fine and NOT flagged: registered fake sql drivers (sql.Open("…fake", …))
# and in-memory miniredis (redis.NewClient(mr.Addr())) carry none of the markers below.
set -euo pipefail
cd "$(dirname "$0")/.."

# Markers that only a real connection uses.
real='sql\.Open\("(postgres|pgx)"|pgxpool\.New|os\.Getenv\("(TEST_)?DATABASE_URL"\)|os\.Getenv\("REDIS_URL"\)|redis\.ParseURL'

violations=0
while IFS= read -r f; do
  # Skip files in the integration / liveuat lanes (build-tagged in the first lines).
  if head -5 "$f" | grep -qE '^//go:build .*(integration|liveuat)'; then
    continue
  fi
  if grep -Eq "$real" "$f"; then
    echo "VIOLATION (unit test opens real infra — add //go:build integration, or split it out): $f"
    grep -nE "$real" "$f" | sed 's/^/    /'
    violations=1
  fi
# Search roots mirror the Makefile's GO_DIRS — keep in sync when a new top-level Go dir is added.
done < <(find backend crm-de cli -name '*_test.go' 2>/dev/null | sort)

if [ "$violations" -ne 0 ]; then
  echo "FAIL: test-lanes — real-infra tests must carry //go:build integration (or liveuat)."
  exit 1
fi
echo "OK: test-lanes — no unit test opens a real DB/Redis"
