-- 000068 — fuzzy dedupe scoring (T07, PO-F-1/PO-F-2). No trigram infra existed anywhere in the 67
-- prior migrations (grep confirmed). The GIN trigram indexes are a coarse candidate-set prefilter
-- for the create-time fuzzy scorer (Go computes the precise Jaro-Winkler score against the
-- candidates this narrows to) — full-table-scanning every live person/org on each create would
-- blow the create budget. Functional index on lower(...), matching the existing
-- organization_domain.domain case-insensitive-lookup idiom, rather than adding a new normalized
-- column.
--
-- DEVIATION (documented, PLAN-REVIEW A1): the spec asks for a trigram index over the
-- "normalized (casefold+unaccent)" columns, but Postgres's unaccent() is not IMMUTABLE and can't
-- be indexed without a custom immutable wrapper function — out of scope for this ticket. The
-- index is on lower(...) only (casefold, no unaccent); the candidate query's normalized search
-- term IS fully unaccented (dedupe.go's normalizeName/normalizeCompanyName), so an accented name
-- can, in the worst case, miss the trigram prefilter for a candidate at a DIFFERENT org (same-org
-- candidates are still caught via the org-sharing OR-branch in Task 3/4's candidate query, which
-- doesn't depend on the trigram operator at all). Precise scoring is unaffected — this only
-- narrows recall of the coarse prefilter for accented cross-org near-duplicates.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_person_full_name_trgm ON person USING gin (lower(full_name) gin_trgm_ops);
CREATE INDEX idx_organization_name_trgm ON organization USING gin (lower(name) gin_trgm_ops);
