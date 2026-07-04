DROP TRIGGER IF EXISTS trg_relationship_touch ON relationship;
ALTER TABLE relationship DROP COLUMN IF EXISTS version;
