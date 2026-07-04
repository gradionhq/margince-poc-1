-- 000066 — realign partner CHECK constraints to PO-DDL-6 target enum values (T15).
-- The partner table (mig 000007) shipped with placeholder enum values that were
-- never correct and never written to any row (the table is empty in seeded/test
-- datasets) — realign directly rather than doing an additive-deprecation dance.

ALTER TABLE partner
  ALTER COLUMN cert_status SET DEFAULT 'applied';

ALTER TABLE partner
  ADD COLUMN IF NOT EXISTS joined_at date NULL;

ALTER TABLE partner
  ADD COLUMN IF NOT EXISTS renews_at date NULL;

ALTER TABLE partner
  DROP CONSTRAINT IF EXISTS partner_cert_status_check,
  ADD CONSTRAINT partner_cert_status_check
    CHECK (cert_status IN ('applied','certified','suspended'));

ALTER TABLE partner
  DROP CONSTRAINT IF EXISTS partner_margin_tier_check,
  ADD CONSTRAINT partner_margin_tier_check
    CHECK (margin_tier IS NULL OR margin_tier IN ('tier1_15','tier2_20','tier3_25'));

ALTER TABLE partner
  ALTER COLUMN certified_staff TYPE smallint;
