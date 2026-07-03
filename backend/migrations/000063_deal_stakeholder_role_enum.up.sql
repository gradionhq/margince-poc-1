-- 000063 — DEAL-EXT-5: relationship.role becomes a CHECK-constrained enum for
-- kind = 'deal_stakeholder' rows (deals-and-pipeline.md#wire--contract-extensions-d-h2).
-- NULL is disallowed for that kind, which also closes uq_rel_deal_person_role's
-- duplicate-NULL-role hole (Postgres unique indexes treat NULL as distinct, so
-- multiple NULL-role stakeholder rows could previously coexist for the same
-- deal+person — now impossible since role can never be NULL for this kind).
-- Employment-kind rows keep free-text role (title vocabulary isn't enumerable),
-- untouched by this CHECK.

ALTER TABLE relationship ADD CONSTRAINT rel_stakeholder_role_enum CHECK (
  kind <> 'deal_stakeholder' OR
  (role IS NOT NULL AND role IN ('champion','economic_buyer','blocker','influencer','user'))
);
