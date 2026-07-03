-- E04 P1 (B-E04.1): an ingested transcript activity points at its blob in object
-- storage via transcript_ref (the blobstore key/ref). Nullable — only call/meeting
-- activities ingested from a transcript carry it.
ALTER TABLE activity ADD COLUMN transcript_ref TEXT;

-- Idempotency for transcript ingest: one transcript-activity per (workspace, source),
-- so a re-delivered capture job upserts the same row instead of duplicating it. The
-- predicate scopes the constraint to transcript rows only — the existing capture path
-- dedups on (source_system, source_id) and may legitimately share a source across rows,
-- so a blanket unique on (workspace_id, source) would wrongly reject those.
CREATE UNIQUE INDEX uq_activity_transcript_source
  ON activity (workspace_id, source) WHERE transcript_ref IS NOT NULL;
