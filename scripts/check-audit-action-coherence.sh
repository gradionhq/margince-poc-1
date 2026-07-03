#!/usr/bin/env bash
# Contract<->DDL coherence gate for the audit_log enums (closes the ARC-C1 drift).
#
# crm.yaml's AuditLogEntry.action / .actor_type are duplicated as Postgres CHECK
# constraints (audit_log_action_check / audit_log_actor_type_check — the effective
# definition is the highest-numbered migration that sets each). When the two copies
# drift, an action the contract defines and code emits (e.g. 'send_email' written by
# crm-core/handler_activity.go) is rejected by the DB at write time — a latent audit
# integrity bug. This gate fails (exit 1) unless both enum sets match exactly.
# crm.yaml is the source of truth (P3): align the DDL CHECK to it (see migration 000047).
set -euo pipefail
root="${1:-.}"
cd "$root"

yaml="backend/api/crm.yaml"
fail=0

# Quoted tokens from the IN-list of the latest migration defining CHECK constraint $1.
ddl_set() {
  local cons="$1" f
  f="$(grep -lE "ADD +CONSTRAINT +$cons" backend/migrations/*.up.sql 2>/dev/null | sort | tail -1 || true)"
  [ -n "${f:-}" ] || return 0
  awk "/ADD +CONSTRAINT +$cons/,/;/" "$f" | grep -oE "'[^']+'" | tr -d "'" | sort -u
}

# The AuditLogEntry schema block (between its 4-space key and the next 4-space key).
audit_block() { awk '/^    AuditLogEntry:/{f=1;next} f&&/^    [A-Za-z]/{f=0} f' "$yaml"; }

# Values from the `[a, b, c]` list on the first enum line in the block matching $1.
yaml_set() {
  audit_block | grep -E "$1" | head -1 | grep -oE '\[[^]]*\]' \
    | tr -d '[]' | tr ',' '\n' | sed 's/ //g' | grep -v '^$' | sort -u
}

cmp_set() { # $1=label  $2=ddl set  $3=contract set
  local label="$1" d="$2" c="$3" miss_ddl miss_con
  if [ -z "$d" ]; then echo "  FAIL  $label: could not parse DDL CHECK"; fail=1; return 0; fi
  if [ -z "$c" ]; then echo "  FAIL  $label: could not parse crm.yaml enum"; fail=1; return 0; fi
  miss_ddl="$(comm -13 <(printf '%s\n' "$d") <(printf '%s\n' "$c"))"
  miss_con="$(comm -23 <(printf '%s\n' "$d") <(printf '%s\n' "$c"))"
  if [ -n "$miss_ddl" ] || [ -n "$miss_con" ]; then
    echo "  FAIL  $label"
    [ -n "$miss_ddl" ] && echo "        DDL CHECK missing (contract defines): $(echo $miss_ddl | tr '\n' ' ')"
    [ -n "$miss_con" ] && echo "        contract enum missing (DDL allows):   $(echo $miss_con | tr '\n' ' ')"
    fail=1
  else
    echo "  OK    $label ($(printf '%s\n' "$d" | grep -c .) values)"
  fi
}

cmp_set "audit_log.action"     "$(ddl_set audit_log_action_check)"     "$(yaml_set 'enum: \[create,')"
cmp_set "audit_log.actor_type" "$(ddl_set audit_log_actor_type_check)" "$(yaml_set 'actor_type:.*enum: \[')"

if [ "$fail" -ne 0 ]; then
  echo ""
  echo "audit_log contract<->DDL drift. crm.yaml is the source of truth (P3): add a"
  echo "migration aligning the CHECK (template: backend/migrations/000047_*), or fix the enum."
  exit 1
fi
echo "audit_log contract<->DDL coherence OK."
