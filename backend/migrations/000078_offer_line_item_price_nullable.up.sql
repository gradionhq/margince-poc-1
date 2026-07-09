BEGIN;
-- 000078 — OP-T07 (OFFER-AC-14/OFFER-AC-11b): the AI drafting/regenerate path must never
-- fabricate a price it cannot ground in the rate-card or the conversation context — an
-- ungrounded line's unit_price_minor is left NULL for the rep to fill in. Pure widening of
-- the existing NOT NULL constraint; every pre-existing row already has a concrete value, so
-- no backfill is needed. (offer_line_item.evidence jsonb already exists since 000071 — no
-- migration needed for it; see this plan's Pre-implementation Finding 1.)
ALTER TABLE offer_line_item ALTER COLUMN unit_price_minor DROP NOT NULL;
COMMIT;
