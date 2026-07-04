-- 000067 — relationship gains version + touch trigger (T08), matching every
-- other mutable core table (person/organization/deal/partner). The contract's
-- Relationship schema already declares `version`; this closes the DDL gap so
-- updateRelationship/archiveRelationship can implement the standard
-- If-Match/version optimistic-concurrency convention. No existing CHECK,
-- index, or RLS policy on relationship changes.
ALTER TABLE relationship ADD COLUMN version bigint NOT NULL DEFAULT 1;
CREATE TRIGGER trg_relationship_touch BEFORE UPDATE ON relationship
  FOR EACH ROW EXECUTE FUNCTION touch_versioned();
