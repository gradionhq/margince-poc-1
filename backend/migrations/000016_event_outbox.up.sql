-- 000016 — Transactional outbox for async event relay to Redis Streams
CREATE TABLE event_outbox (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    topic        text NOT NULL,
    entity_id    uuid NOT NULL,
    payload      jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);

ALTER TABLE event_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_outbox FORCE ROW LEVEL SECURITY;

CREATE POLICY event_outbox_ws ON event_outbox
    USING (workspace_id::text = current_setting('app.workspace_id', true));

CREATE INDEX event_outbox_unpublished ON event_outbox (created_at)
    WHERE published_at IS NULL;
