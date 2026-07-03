BEGIN;

-- Global reference table: consent purposes (no workspace_id, no RLS — lookup only).
CREATE TABLE consent_purpose (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  name        text NOT NULL UNIQUE,
  description text NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO consent_purpose (name, description) VALUES
  ('marketing_email',  'Email marketing communications'),
  ('marketing_phone',  'Phone marketing communications'),
  ('profiling',        'Profiling and personalisation'),
  ('product_updates',  'Product update notifications');

-- Per-workspace current consent state (mutable; append-only proof lives in consent_event).
CREATE TABLE person_consent (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id   uuid NOT NULL REFERENCES workspace(id) ON DELETE RESTRICT,
  person_id      uuid NOT NULL REFERENCES person(id)   ON DELETE CASCADE,
  purpose_id     uuid NOT NULL REFERENCES consent_purpose(id) ON DELETE RESTRICT,
  state          text NOT NULL DEFAULT 'unknown'
                   CHECK (state IN ('granted','withdrawn','unknown')),
  lawful_basis   text NULL,
  captured_at    timestamptz NOT NULL DEFAULT now(),
  source         text NOT NULL,
  policy_version text NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, person_id, purpose_id)
);
CREATE INDEX idx_person_consent_ws         ON person_consent (workspace_id);
CREATE INDEX idx_person_consent_person     ON person_consent (person_id);
CREATE INDEX idx_person_consent_purpose    ON person_consent (purpose_id);

ALTER TABLE person_consent ENABLE ROW LEVEL SECURITY;
ALTER TABLE person_consent FORCE ROW LEVEL SECURITY;
CREATE POLICY person_consent_tenant_isolation ON person_consent
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

CREATE TRIGGER trg_person_consent_updated
  BEFORE UPDATE ON person_consent
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Append-only cryptographic proof log: every consent signal is preserved forever.
CREATE TABLE consent_event (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  workspace_id     uuid NOT NULL REFERENCES workspace(id)        ON DELETE RESTRICT,
  person_id        uuid NOT NULL REFERENCES person(id)           ON DELETE CASCADE,
  purpose_id       uuid NOT NULL REFERENCES consent_purpose(id)  ON DELETE RESTRICT,
  event_state      text NOT NULL CHECK (event_state IN ('granted','withdrawn')),
  channel          text NULL,
  lawful_basis     text NULL,
  policy_wording   text NOT NULL,
  policy_version   text NOT NULL,
  double_opt_in_ref text NULL,
  source           text NOT NULL,
  occurred_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_consent_event_ws      ON consent_event (workspace_id);
CREATE INDEX idx_consent_event_person  ON consent_event (person_id);
CREATE INDEX idx_consent_event_purpose ON consent_event (purpose_id);

ALTER TABLE consent_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE consent_event FORCE ROW LEVEL SECURITY;
CREATE POLICY consent_event_tenant_isolation ON consent_event
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);

-- Append-only enforcement: mirror audit_log_immutable() pattern exactly.
CREATE OR REPLACE FUNCTION consent_event_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'consent_event is append-only (attempted % on row %)', TG_OP, OLD.id
    USING ERRCODE = 'check_violation';
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_consent_event_no_mutate
  BEFORE UPDATE OR DELETE ON consent_event
  FOR EACH ROW EXECUTE FUNCTION consent_event_immutable();

GRANT SELECT, INSERT, UPDATE, DELETE ON person_consent  TO margince_app;
GRANT SELECT, INSERT               ON consent_event     TO margince_app;
GRANT SELECT                       ON consent_purpose   TO margince_app;

COMMIT;
