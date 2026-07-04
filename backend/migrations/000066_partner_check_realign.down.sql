ALTER TABLE partner
  DROP COLUMN IF EXISTS renews_at;

ALTER TABLE partner
  DROP COLUMN IF EXISTS joined_at;

ALTER TABLE partner
  ALTER COLUMN certified_staff TYPE integer;

ALTER TABLE partner
  DROP CONSTRAINT IF EXISTS partner_margin_tier_check,
  ADD CONSTRAINT partner_margin_tier_check
    CHECK (margin_tier IS NULL OR margin_tier IN ('standard','silver','gold','platinum'));

ALTER TABLE partner
  DROP CONSTRAINT IF EXISTS partner_cert_status_check,
  ADD CONSTRAINT partner_cert_status_check
    CHECK (cert_status IN ('pending','active','suspended','expired'));

ALTER TABLE partner
  ALTER COLUMN cert_status SET DEFAULT 'pending';
