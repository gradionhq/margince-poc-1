-- 000008 — deal_stage_history + fx_rate + lead improvements (WP1 §6.4 + §8)

-- deal_stage_history (append-only snapshot, data-model §6.4)
CREATE TABLE deal_stage_history (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id   uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  deal_id        uuid NOT NULL REFERENCES deal(id) ON DELETE CASCADE,
  from_stage_id  uuid NULL REFERENCES stage(id) ON DELETE SET NULL,
  to_stage_id    uuid NOT NULL REFERENCES stage(id) ON DELETE RESTRICT,
  changed_by     text NOT NULL,
  amount_minor_at_change bigint NULL,
  currency_at_change     char(3) NULL,
  occurred_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_deal_stage_history_deal ON deal_stage_history (deal_id, occurred_at DESC);
CREATE INDEX idx_deal_stage_history_ws   ON deal_stage_history (workspace_id, occurred_at DESC);
ALTER TABLE deal_stage_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE deal_stage_history FORCE ROW LEVEL SECURITY;
CREATE POLICY deal_stage_history_tenant_isolation ON deal_stage_history
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- fx_rate (data-model §6.4 / data-semantics §1.2)
CREATE TABLE fx_rate (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  from_currency char(3) NOT NULL CHECK (from_currency ~ '^[A-Z]{3}$'),
  to_currency   char(3) NOT NULL CHECK (to_currency ~ '^[A-Z]{3}$'),
  rate          numeric(20,10) NOT NULL,
  rate_date     date NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_fx_rate UNIQUE (workspace_id, from_currency, to_currency, rate_date)
);
CREATE INDEX idx_fx_rate_lookup ON fx_rate (workspace_id, from_currency, to_currency, rate_date DESC);
ALTER TABLE fx_rate ENABLE ROW LEVEL SECURITY;
ALTER TABLE fx_rate FORCE ROW LEVEL SECURITY;
CREATE POLICY fx_rate_tenant_isolation ON fx_rate
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- lead improvements: promoted_person_id, search_tsv, idempotent source (data-model §8)
ALTER TABLE lead
  ADD COLUMN promoted_person_id uuid NULL REFERENCES person(id) ON DELETE SET NULL,
  ADD COLUMN promoted_at timestamptz NULL,
  ADD COLUMN source_system text NULL,
  ADD COLUMN source_id text NULL,
  ADD COLUMN search_tsv tsvector GENERATED ALWAYS AS (
    to_tsvector('simple',
      coalesce(full_name,'') || ' ' ||
      coalesce(email,'') || ' ' ||
      coalesce(company_name,''))
  ) STORED;

CREATE UNIQUE INDEX uq_lead_source ON lead (workspace_id, source_system, source_id)
  WHERE source_system IS NOT NULL AND source_id IS NOT NULL;
CREATE INDEX idx_lead_promoted ON lead (workspace_id, promoted_person_id)
  WHERE promoted_person_id IS NOT NULL;
CREATE INDEX idx_lead_score ON lead (workspace_id, score DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_lead_search ON lead USING gin (search_tsv);
