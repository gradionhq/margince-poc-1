DROP TABLE IF EXISTS fx_rate, deal_stage_history CASCADE;
DROP INDEX IF EXISTS idx_lead_search;
DROP INDEX IF EXISTS idx_lead_score;
DROP INDEX IF EXISTS idx_lead_promoted;
DROP INDEX IF EXISTS uq_lead_source;
ALTER TABLE lead
  DROP COLUMN IF EXISTS search_tsv,
  DROP COLUMN IF EXISTS source_id,
  DROP COLUMN IF EXISTS source_system,
  DROP COLUMN IF EXISTS promoted_at,
  DROP COLUMN IF EXISTS promoted_person_id;
