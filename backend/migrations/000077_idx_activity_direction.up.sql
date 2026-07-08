BEGIN;
-- 000077 — AT-T03/ACT-DDL-1: the only new activity index this ticket adds. ACT-DDL-1's other
-- indexes (idx_activity_ws_time, idx_activity_kind, idx_activity_tasks, idx_activity_search) already
-- exist (migration 000003) and are NOT recreated here. Mirrors idx_activity_kind's exact shape,
-- swapping kind for direction — supports a direction-filtered, time-ordered scan. No crm.yaml query
-- param surfaces `direction` yet (see plan Global Constraints #3); the store-layer filter carries it
-- for forward-compatibility and this index is what a direction predicate would use once it does.
CREATE INDEX idx_activity_direction ON activity (workspace_id, direction, occurred_at DESC)
  WHERE archived_at IS NULL;
COMMIT;
