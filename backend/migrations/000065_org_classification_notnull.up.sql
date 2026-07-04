-- 000065 — organization.classification NOT NULL DEFAULT 'prospect' (PO-DDL-4).
-- Backfill existing NULL rows before adding the constraint so the ALTER never
-- fails against live data; relevance (smallint 0-100 nullable) is already
-- correct per migration 000006 and is untouched here.
UPDATE organization SET classification = 'prospect' WHERE classification IS NULL;

ALTER TABLE organization
  ALTER COLUMN classification SET DEFAULT 'prospect',
  ALTER COLUMN classification SET NOT NULL;
