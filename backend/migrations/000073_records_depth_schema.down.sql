-- 000073 down — reverse of the up migration. No inter-table FK ordering constraint among the
-- three tables (none references another); plain DROP TABLE also drops each table's own
-- indexes, policies, and triggers. attachment predates this migration (000009) and is only
-- reconciled via ALTER here, so its reversal is the exact inverse ALTER (restore the 000009
-- shape) rather than a drop — dropping it would destroy a pre-existing table this migration
-- doesn't own.
DROP TABLE IF EXISTS bulk_operation;
DROP TABLE IF EXISTS quota;

DROP TRIGGER IF EXISTS trg_attachment_updated ON attachment;
ALTER TABLE attachment DROP COLUMN updated_at;
ALTER TABLE attachment ALTER COLUMN content_type SET NOT NULL;
ALTER TABLE attachment ALTER COLUMN byte_size SET NOT NULL;
