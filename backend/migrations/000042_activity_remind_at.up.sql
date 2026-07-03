-- 000042 — activity_remind_at: add remind_at timestamptz (task-only) + extend activity_task_fields CHECK.
-- A named CHECK cannot be altered in place; we drop + re-add with the new condition.
-- The original CHECK (from 000003_core_objects):
--   CONSTRAINT activity_task_fields CHECK (kind = 'task' OR (due_at IS NULL AND assignee_id IS NULL AND is_done = false))
-- The new CHECK adds remind_at IS NULL for non-task rows.

ALTER TABLE activity
  ADD COLUMN remind_at timestamptz NULL;

ALTER TABLE activity
  DROP CONSTRAINT activity_task_fields;

ALTER TABLE activity
  ADD CONSTRAINT activity_task_fields CHECK (
    kind = 'task' OR (due_at IS NULL AND assignee_id IS NULL AND is_done = false AND remind_at IS NULL)
  );
