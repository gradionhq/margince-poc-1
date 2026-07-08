-- 000071 — offers & products (OP-T03, OFFER-DDL-1..4 verbatim): product catalogue, the
-- versioned offer (Angebot) record, its line items, and PDF-layout templates. Schema-only
-- migration — no handler/store/contract code (ticket OP-T01 owns the wire surface,
-- independently, no code dependency either direction).
--
-- Statement order: product, offer_template, offer, offer_line_item. offer.template_id
-- references offer_template(id), so offer_template must exist before offer (OFFER-DDL-2's
-- forward-reference note); product/offer must exist before offer_line_item (its product_id/
-- offer_id FKs). This is the only inter-table ordering constraint among the four.
--
-- Per OFFER-DDL-N-1: no invoice table (germany-package/E17, far future) and no accept/view/
-- open tracking table (reuses deal-rooms' deal_room_engagement_event, owned elsewhere) —
-- both explicitly out of scope.

-- OFFER-DDL-1 — product (optional rate-card / catalogue entry; a workspace may quote fully
-- free-form without ever using this table)
CREATE TABLE product (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id      uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name              text NOT NULL,
  sku               text NULL,
  description       text NULL,
  unit              text NOT NULL DEFAULT 'unit',
  unit_price_minor  bigint NOT NULL,
  currency          char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  default_tax_rate  numeric(5,2) NOT NULL DEFAULT 0,
  active            boolean NOT NULL DEFAULT true,
  version           bigint NOT NULL DEFAULT 1,
  source            text NOT NULL,
  captured_by       text NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  archived_at       timestamptz NULL
);
CREATE UNIQUE INDEX uq_product_sku ON product (workspace_id, sku) WHERE sku IS NOT NULL AND archived_at IS NULL;
CREATE INDEX idx_product_active ON product (workspace_id, active) WHERE archived_at IS NULL;
CREATE TRIGGER trg_product_touch BEFORE UPDATE ON product FOR EACH ROW EXECUTE FUNCTION touch_versioned();

ALTER TABLE product ENABLE ROW LEVEL SECURITY;
ALTER TABLE product FORCE ROW LEVEL SECURITY;
CREATE POLICY product_tenant_isolation ON product
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON product TO margince_app;

-- OFFER-DDL-4 — offer_template (branded, governed PDF layout, DE/EN). Created before `offer`
-- because offer.template_id references it. No `source`/`captured_by` — unlike DDL-1/DDL-2,
-- DDL-4's pinned schema does not list them (workspace-authored config, not captured data).
CREATE TABLE offer_template (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  name          text NOT NULL,
  locale        text NOT NULL DEFAULT 'de-DE',
  is_default    boolean NOT NULL DEFAULT false,
  layout        jsonb NOT NULL,
  version       bigint NOT NULL DEFAULT 1,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  archived_at   timestamptz NULL,
  CONSTRAINT offer_template_name_unique UNIQUE (workspace_id, name)
);
CREATE UNIQUE INDEX uq_offer_template_default ON offer_template (workspace_id, locale) WHERE is_default AND archived_at IS NULL;
CREATE TRIGGER trg_offer_template_touch BEFORE UPDATE ON offer_template FOR EACH ROW EXECUTE FUNCTION touch_versioned();

ALTER TABLE offer_template ENABLE ROW LEVEL SECURITY;
ALTER TABLE offer_template FORCE ROW LEVEL SECURITY;
CREATE POLICY offer_template_tenant_isolation ON offer_template
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON offer_template TO margince_app;

-- OFFER-DDL-2 — offer (a versioned Angebot bound to one deal)
CREATE TABLE offer (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id    uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  deal_id         uuid NOT NULL REFERENCES deal(id) ON DELETE RESTRICT,
  offer_number    text NOT NULL,
  revision        integer NOT NULL DEFAULT 1,
  status          text NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft','sent','accepted','rejected','expired','superseded')),
  currency        char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
  buyer_org_id    uuid NULL REFERENCES organization(id) ON DELETE SET NULL,
  buyer_snapshot  jsonb NULL,
  issuer_snapshot jsonb NULL,
  valid_until     date NULL,
  intro_text      text NULL,
  terms_text      text NULL,
  net_minor       bigint NOT NULL DEFAULT 0,
  tax_minor       bigint NOT NULL DEFAULT 0,
  gross_minor     bigint NOT NULL DEFAULT 0,
  fx_rate_to_base numeric(20,10) NULL,
  fx_rate_date    date NULL,
  template_id     uuid NULL REFERENCES offer_template(id) ON DELETE SET NULL,
  pdf_asset_ref   text NULL,
  accepted_at     timestamptz NULL,
  version         bigint NOT NULL DEFAULT 1,
  source          text NOT NULL,
  captured_by     text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  archived_at     timestamptz NULL,
  CONSTRAINT offer_number_rev_unique UNIQUE (workspace_id, offer_number, revision),
  CONSTRAINT offer_accepted_at CHECK (status <> 'accepted' OR accepted_at IS NOT NULL)
);
CREATE INDEX idx_offer_deal ON offer (workspace_id, deal_id, revision DESC) WHERE archived_at IS NULL;
CREATE INDEX idx_offer_status ON offer (workspace_id, status) WHERE archived_at IS NULL;
CREATE TRIGGER trg_offer_touch BEFORE UPDATE ON offer FOR EACH ROW EXECUTE FUNCTION touch_versioned();

ALTER TABLE offer ENABLE ROW LEVEL SECURITY;
ALTER TABLE offer FORCE ROW LEVEL SECURITY;
CREATE POLICY offer_tenant_isolation ON offer
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON offer TO margince_app;

-- OFFER-DDL-3 — offer_line_item (a typed line; price snapshot copied from product at add-
-- time). No `version`/touch trigger — DDL-3 has no "+ version" suffix; updated_at is kept
-- fresh by set_updated_at() (mirrors the non-versioned person_email table, 000003). No
-- line_net/line_tax/line_total columns — those are derived in code only (OFFER-PARAM-4).
CREATE TABLE offer_line_item (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id      uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  offer_id          uuid NOT NULL REFERENCES offer(id) ON DELETE CASCADE,
  position          integer NOT NULL,
  product_id        uuid NULL REFERENCES product(id) ON DELETE SET NULL,
  description       text NOT NULL,
  unit              text NOT NULL DEFAULT 'unit',
  quantity          numeric(14,3) NOT NULL CHECK (quantity > 0),
  unit_price_minor  bigint NOT NULL,
  discount_pct      numeric(5,2) NOT NULL DEFAULT 0 CHECK (discount_pct BETWEEN 0 AND 100),
  tax_rate          numeric(5,2) NOT NULL DEFAULT 0,
  evidence          jsonb NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  archived_at       timestamptz NULL,
  CONSTRAINT offer_line_item_position_unique UNIQUE (offer_id, position)
);
CREATE INDEX idx_oli_offer ON offer_line_item (offer_id, position);
CREATE TRIGGER trg_offer_line_item_updated BEFORE UPDATE ON offer_line_item FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE offer_line_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE offer_line_item FORCE ROW LEVEL SECURITY;
CREATE POLICY offer_line_item_tenant_isolation ON offer_line_item
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON offer_line_item TO margince_app;
