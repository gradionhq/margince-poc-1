DROP INDEX IF EXISTS uq_activity_transcript_source;
ALTER TABLE activity DROP COLUMN IF EXISTS transcript_ref;
