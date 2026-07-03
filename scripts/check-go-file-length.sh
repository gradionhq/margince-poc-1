#!/usr/bin/env bash
# T1 file-length gate — architecture/18-code-quality-operating-model.md §3.2.
# File-length is not a built-in golangci linter, so the ~400-500 LOC §15 §3 smell
# threshold is enforced here: a per-file hard cap over *.go, excluding generated
# (_gen.go / .gen.go) and test (_test.go) files. A file above the cap is a god-file
# split candidate (the PoC's 1,970-LOC store.go). Wired into `make check-go` (T1).
set -euo pipefail

CAP="${GO_FILE_LINE_CAP:-500}"
DIRS=(backend crm-de cli)

offenders=$(
  find "${DIRS[@]}" -name "*.go" \
    ! -name "*_test.go" ! -name "*_gen.go" ! -name "*.gen.go" 2>/dev/null \
    | xargs wc -l 2>/dev/null \
    | awk -v cap="$CAP" '$2 != "total" && $1 > cap {printf "  %5d  %s\n", $1, $2}' \
    | sort -rn
)

if [ -n "$offenders" ]; then
  echo "FAIL: Go file(s) exceed the ${CAP}-LOC cap (architecture/18 §3.2 — split into one-concept-per-file packages):"
  echo "$offenders"
  exit 1
fi
echo "OK: no Go file exceeds ${CAP} LOC"
