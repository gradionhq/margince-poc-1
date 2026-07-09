BEGIN;
-- 000078 — OP-T07 (OFFER-AC-14/OFFER-AC-11b): the AI drafting/regenerate path must never
-- fabricate a price it cannot ground in the rate-card or the conversation context. Rather than
-- widen the existing unit_price_minor column to nullable (a confirmed contract-breaking change —
-- oasdiff classifies an existing response property becoming nullable as ERR), this adds a
-- purely additive boolean signal instead: price_grounded=false means unit_price_minor is an
-- honest 0 sentinel (never a guessed nonzero value) that the rep must fill in; true (the
-- column's own default) is every pre-existing / human-entered / rate-card-grounded line,
-- unchanged. (offer_line_item.evidence jsonb already exists since 000071 — no migration needed
-- for it; see this plan's Pre-implementation Finding 1.)
ALTER TABLE offer_line_item ADD COLUMN price_grounded boolean NOT NULL DEFAULT true;
COMMIT;
