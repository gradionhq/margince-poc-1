DROP INDEX IF EXISTS event_outbox_unpublished;
DROP POLICY IF EXISTS event_outbox_ws ON event_outbox;
DROP TABLE IF EXISTS event_outbox;
