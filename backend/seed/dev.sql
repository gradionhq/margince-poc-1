-- Dev seed data for Margince
-- Idempotent: safe to run multiple times (ON CONFLICT DO NOTHING / DO UPDATE)
-- Run: make seed FILE=dev.sql
--
-- Dev workspace ID: 00000000-0000-0000-0000-000000000001
-- Vite proxy sends this ID as X-Workspace-ID for local dev (until WP6 auth lands)

BEGIN;

-- ─── Workspace ──────────────────────────────────────────────

INSERT INTO workspace (id, name, slug, base_currency)
VALUES ('00000000-0000-0000-0000-000000000001', 'Dev Workspace', 'dev', 'EUR')
ON CONFLICT (id) DO NOTHING;

-- ─── Consent purposes (D2 / migration 000070_ws_c_conformance) ─────
-- consent_purpose went per-workspace in 000070; its own backfill clones
-- existing workspaces' purposes at MIGRATE time, which runs before this seed
-- ever creates the dev workspace, so it clones 0 rows for it. The signup
-- handler (auth_handler.go's defaultConsentPurposes) seeds these 4 for any
-- newly-signed-up workspace, but the dev workspace is inserted via raw SQL
-- above, bypassing that path too — so it must be seeded here, mirroring the
-- same 4 keys/labels.
INSERT INTO consent_purpose (workspace_id, key, label)
VALUES
  ('00000000-0000-0000-0000-000000000001', 'marketing_email', 'Email marketing communications'),
  ('00000000-0000-0000-0000-000000000001', 'marketing_phone', 'Phone marketing communications'),
  ('00000000-0000-0000-0000-000000000001', 'profiling',       'Profiling and personalisation'),
  ('00000000-0000-0000-0000-000000000001', 'product_updates', 'Product update notifications')
ON CONFLICT (workspace_id, key) DO UPDATE SET label = EXCLUDED.label;

-- ─── Sample people ──────────────────────────────────────────

INSERT INTO person (id, workspace_id, full_name, source, captured_by)
VALUES
  ('00000000-0000-0000-0001-000000000001', '00000000-0000-0000-0000-000000000001',
   'Alice Müller', 'seed', 'human:dev'),
  ('00000000-0000-0000-0001-000000000002', '00000000-0000-0000-0000-000000000001',
   'Bob Schmidt', 'seed', 'human:dev'),
  ('00000000-0000-0000-0001-000000000003', '00000000-0000-0000-0000-000000000001',
   'Carol Wagner', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- ─── Sample emails ──────────────────────────────────────────

INSERT INTO person_email (id, person_id, workspace_id, email, email_type, is_primary, position, source, captured_by)
VALUES
  ('00000000-0000-0000-0002-000000000001', '00000000-0000-0000-0001-000000000001',
   '00000000-0000-0000-0000-000000000001', 'alice@example.com', 'work', true, 0, 'seed', 'human:dev'),
  ('00000000-0000-0000-0002-000000000002', '00000000-0000-0000-0001-000000000002',
   '00000000-0000-0000-0000-000000000001', 'bob@example.com', 'work', true, 0, 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- ─── Auth seed (EP03) ───────────────────────────────────────

-- Admin user: admin@example.com / changeme
-- bcrypt hash of "changeme" at cost 10
INSERT INTO app_user (id, workspace_id, email, display_name, password_hash)
VALUES (
  '00000000-0000-0000-0010-000000000001',
  '00000000-0000-0000-0000-000000000001',
  'admin@example.com',
  'Admin User',
  '$2a$10$bclrO7qYuxFBHUyKVkYGu.dpaZXWcg/u3S5NnQsBX75VLBD.3j2tu'
) ON CONFLICT DO NOTHING;

-- Rep user: rep@example.com / changeme
INSERT INTO app_user (id, workspace_id, email, display_name, password_hash)
VALUES (
  '00000000-0000-0000-0010-000000000002',
  '00000000-0000-0000-0000-000000000001',
  'rep@example.com',
  'Rep User',
  '$2a$10$bclrO7qYuxFBHUyKVkYGu.dpaZXWcg/u3S5NnQsBX75VLBD.3j2tu'
) ON CONFLICT DO NOTHING;

-- Read-only user: readonly@example.com / changeme
INSERT INTO app_user (id, workspace_id, email, display_name, password_hash)
VALUES (
  '00000000-0000-0000-0010-000000000003',
  '00000000-0000-0000-0000-000000000001',
  'readonly@example.com',
  'Read Only User',
  '$2a$10$bclrO7qYuxFBHUyKVkYGu.dpaZXWcg/u3S5NnQsBX75VLBD.3j2tu'
) ON CONFLICT DO NOTHING;

-- Manager user: manager@example.com / changeme (required for live-UAT curate step)
INSERT INTO app_user (id, workspace_id, email, display_name, password_hash)
VALUES (
  '00000000-0000-0000-0010-000000000004',
  '00000000-0000-0000-0000-000000000001',
  'manager@example.com',
  'Manager User',
  '$2a$10$bclrO7qYuxFBHUyKVkYGu.dpaZXWcg/u3S5NnQsBX75VLBD.3j2tu'
) ON CONFLICT DO NOTHING;

-- ─── Roles (EP03) ───────────────────────────────────────────

INSERT INTO role (id, workspace_id, key, is_system, permissions) VALUES
(
  '00000000-0000-0000-0020-000000000001',
  '00000000-0000-0000-0000-000000000001',
  'admin', true,
  '{"person":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"organization":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"relationship":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"pipeline":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"stage":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"partner":{"read":{"row_scope":"all"},"update":{"row_scope":"all"}},"activity":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"lead":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"product":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"offer_template":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"offer":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"attachment":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"invoice":{"read":{"row_scope":"all"},"create":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}},"passport":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"approval":{"read":{"row_scope":"all"},"decide":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"},"curate":{"row_scope":"all"}},"conversation_link":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal_room":{"publish":{"row_scope":"all"}},"workspace":{"manage_members":{"row_scope":"all"},"export":{"row_scope":"all"},"import":{"row_scope":"all"}},"automation":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"custom_field":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"quota":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"computed_field":{"read":{"row_scope":"all"}}}'
),
(
  '00000000-0000-0000-0020-000000000002',
  '00000000-0000-0000-0000-000000000001',
  'rep', false,
  '{"person":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"organization":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"deal":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"relationship":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"pipeline":{"read":{"row_scope":"own"}},"stage":{"read":{"row_scope":"all"}},"activity":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"lead":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"product":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"offer_template":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"offer":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"attachment":{"read":{"row_scope":"own"},"create":{"row_scope":"own"}},"invoice":{"read":{"row_scope":"own"},"create":{"row_scope":"own"}},"approval":{"read":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"conversation_link":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"archive":{"row_scope":"own"}},"automation":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"quota":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"computed_field":{"read":{"row_scope":"all"}}}'
),
(
  '00000000-0000-0000-0020-000000000003',
  '00000000-0000-0000-0000-000000000001',
  'read_only', false,
  '{"person":{"read":{"row_scope":"all"}},"organization":{"read":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"}},"relationship":{"read":{"row_scope":"all"}},"stage":{"read":{"row_scope":"all"}},"activity":{"read":{"row_scope":"all"}},"lead":{"read":{"row_scope":"all"}},"product":{"read":{"row_scope":"all"}},"offer_template":{"read":{"row_scope":"all"}},"offer":{"read":{"row_scope":"all"}},"attachment":{"read":{"row_scope":"all"}},"invoice":{"read":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}},"approval":{"read":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"all"}},"automation":{"read":{"row_scope":"all"}},"quota":{"read":{"row_scope":"all"}},"computed_field":{"read":{"row_scope":"all"}}}'
),
(
  '00000000-0000-0000-0020-000000000004',
  '00000000-0000-0000-0000-000000000001',
  'manager', false,
  '{"person":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"organization":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"relationship":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"stage":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"activity":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"lead":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"product":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"offer_template":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"offer":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"attachment":{"read":{"row_scope":"all"},"create":{"row_scope":"all"}},"invoice":{"read":{"row_scope":"all"},"create":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}},"approval":{"read":{"row_scope":"all"},"decide":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"},"curate":{"row_scope":"all"}},"conversation_link":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal_room":{"publish":{"row_scope":"all"}},"workspace":{"export":{"row_scope":"all"},"import":{"row_scope":"all"}},"automation":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"quota":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"computed_field":{"read":{"row_scope":"all"}}}'
),
(
  '00000000-0000-0000-0020-000000000005',
  '00000000-0000-0000-0000-000000000001',
  'ops', false,
  '{"report":{"read":{"row_scope":"all"}}}'
)
ON CONFLICT (workspace_id, key) DO UPDATE SET permissions = EXCLUDED.permissions;

-- Role assignments
INSERT INTO role_assignment (id, workspace_id, role_id, user_id) VALUES
('00000000-0000-0000-0030-000000000001','00000000-0000-0000-0000-000000000001',
 '00000000-0000-0000-0020-000000000001','00000000-0000-0000-0010-000000000001'),
('00000000-0000-0000-0030-000000000002','00000000-0000-0000-0000-000000000001',
 '00000000-0000-0000-0020-000000000002','00000000-0000-0000-0010-000000000002'),
('00000000-0000-0000-0030-000000000003','00000000-0000-0000-0000-000000000001',
 '00000000-0000-0000-0020-000000000003','00000000-0000-0000-0010-000000000003'),
('00000000-0000-0000-0030-000000000004','00000000-0000-0000-0000-000000000001',
 '00000000-0000-0000-0020-000000000004','00000000-0000-0000-0010-000000000004')
ON CONFLICT DO NOTHING;

-- ─── Sales pipeline + stages ────────────────────────────────
-- Seven pinned stages per DEAL-FORM-1: New/10, Qualified/25, Discovery/40,
-- Proposal/60, Negotiation/80, Closed Won/100, Closed Lost/0. Stage IDs
-- ...041-000000000001 and ...041-000000000002 are FK-referenced by the
-- seeded deal rows below — kept stable, only name/position/win_probability
-- retuned to match the pinned values.

INSERT INTO pipeline (id, workspace_id, name, is_default, position)
VALUES ('00000000-0000-0000-0040-000000000001', '00000000-0000-0000-0000-000000000001',
        'Sales Pipeline', true, 1)
ON CONFLICT (id) DO NOTHING;

-- semantic ∈ {open,won,lost}; won ⇒ prob 100, lost ⇒ prob 0 (stage_terminal_prob).
INSERT INTO stage (id, workspace_id, pipeline_id, name, position, semantic, win_probability)
VALUES
  ('00000000-0000-0000-0041-000000000001', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'New',         1, 'open', 10),
  ('00000000-0000-0000-0041-000000000002', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Qualified',   2, 'open', 25),
  ('00000000-0000-0000-0041-000000000003', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Discovery',   3, 'open', 40),
  ('00000000-0000-0000-0041-000000000004', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Proposal',    4, 'open', 60),
  ('00000000-0000-0000-0041-000000000005', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Negotiation', 5, 'open', 80),
  ('00000000-0000-0000-0041-000000000006', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Closed Won',  6, 'won',  100),
  ('00000000-0000-0000-0041-000000000007', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0040-000000000001', 'Closed Lost', 7, 'lost', 0)
ON CONFLICT (id) DO NOTHING;

-- ─── Sample organization ─────────────────────────────────────
-- T21 live-UAT needs at least one seeded organization to link deals to.

INSERT INTO organization (id, workspace_id, name, source, captured_by)
VALUES ('00000000-0000-0000-0044-000000000001', '00000000-0000-0000-0000-000000000001',
        'Acme Corp', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- ─── Sample deals (one owned by rep, one by admin) ──────────
-- Gives the EP03 row-scope guides "own" vs "all" data to work with. T21 adds
-- one open deal per remaining open stage (Discovery/Proposal/Negotiation) plus
-- a genuinely stalled deal, per workspace/manual-test/t21.md's prereqs.

INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, owner_id, organization_id, amount_minor, currency, source, captured_by)
VALUES
  ('00000000-0000-0000-0042-000000000001', '00000000-0000-0000-0000-000000000001',
   'Acme Expansion', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000001', '00000000-0000-0000-0010-000000000002',
   '00000000-0000-0000-0044-000000000001', 500000, 'EUR', 'seed', 'human:dev'),
  ('00000000-0000-0000-0042-000000000002', '00000000-0000-0000-0000-000000000001',
   'Globex Renewal', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000002', '00000000-0000-0000-0010-000000000001',
   NULL, 750000, 'EUR', 'seed', 'human:dev'),
  ('00000000-0000-0000-0042-000000000003', '00000000-0000-0000-0000-000000000001',
   'Initech Discovery Call', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000003', '00000000-0000-0000-0010-000000000002',
   NULL, 300000, 'EUR', 'seed', 'human:dev'),
  ('00000000-0000-0000-0042-000000000004', '00000000-0000-0000-0000-000000000001',
   'Umbrella Proposal', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000004', '00000000-0000-0000-0010-000000000002',
   -- T22 live-UAT: needs organization_id set so guide step 1's "View company"
   -- link has something to point at on the same fixture that also carries the
   -- multi-stakeholder framing (champion + economic_buyer, below).
   '00000000-0000-0000-0044-000000000001', 900000, 'EUR', 'seed', 'human:dev'),
  ('00000000-0000-0000-0042-000000000005', '00000000-0000-0000-0000-000000000001',
   'Stark Negotiation', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000005', '00000000-0000-0000-0010-000000000002',
   NULL, 1200000, 'EUR', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- Back-fill last_activity_at so the "Stark Negotiation" deal is genuinely
-- stalled (DEAL-FORM-3: open + idle > 60 days, no wait_until suppression) —
-- ON CONFLICT DO NOTHING on the INSERT above can't set this, so it's a
-- separate idempotent UPDATE guarded by a WHERE that only ever tightens.
UPDATE deal SET last_activity_at = now() - interval '90 days'
WHERE id = '00000000-0000-0000-0042-000000000005'
  AND (last_activity_at IS NULL OR last_activity_at > now() - interval '60 days');

-- ─── Deal stakeholder relationships (B-E09.9 / T21 UAT seed) ─
-- Gives the stakeholders-per-deal cross-object report + its "Explain This Number"
-- drill-through a non-empty result to resolve (swarm/manual-test/b-e09-9.md Step 4),
-- and gives T21's board a single-threaded (exactly one stakeholder) deal to flag.

INSERT INTO relationship (id, workspace_id, kind, person_id, deal_id, role, source, captured_by)
VALUES
  ('00000000-0000-0000-0043-000000000001', '00000000-0000-0000-0000-000000000001',
   'deal_stakeholder', '00000000-0000-0000-0001-000000000001',
   '00000000-0000-0000-0042-000000000001', 'champion', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- ─── Audit log rows (E11-P7 UAT seed) ──────────────────────
-- Provides observable history for the seeded deal and person so that
-- GET /records/{entity_type}/{id}/history returns ≥ 1 entry in live UAT.

INSERT INTO audit_log (workspace_id, actor_type, actor_id, action, entity_type, entity_id, before, after)
VALUES
  -- Deal: Acme Expansion — create
  (
    '00000000-0000-0000-0000-000000000001'::uuid,
    'human',
    '00000000-0000-0000-0010-000000000001',
    'create',
    'deal',
    '00000000-0000-0000-0042-000000000001'::uuid,
    NULL,
    '{"name":"Acme Expansion","status":"open"}'::jsonb
  ),
  -- Deal: Acme Expansion — stage update
  (
    '00000000-0000-0000-0000-000000000001'::uuid,
    'human',
    '00000000-0000-0000-0010-000000000001',
    'update',
    'deal',
    '00000000-0000-0000-0042-000000000001'::uuid,
    '{"stage":"Lead"}'::jsonb,
    '{"stage":"Qualified"}'::jsonb
  ),
  -- Person: Alice Müller — create
  (
    '00000000-0000-0000-0000-000000000001'::uuid,
    'human',
    '00000000-0000-0000-0010-000000000001',
    'create',
    'person',
    '00000000-0000-0000-0001-000000000001'::uuid,
    NULL,
    '{"full_name":"Alice Müller"}'::jsonb
  );

-- ─── T22 live-UAT fixtures (Deal 360) ───────────────────────
-- Guide workspace/manual-test/t22.md needs: an open mid-pipeline deal with
-- 2+ stakeholders (one economic_buyer), a closed (won/lost) deal, a deal
-- with zero stakeholders (already satisfied by Initech/Wonka/Wayne having
-- no relationship rows below), and a deal with logged email/call/meeting
-- activities plus an open task assigned to a seeded app_user. Mirrors
-- T21's precedent of extending dev.sql for its own guide's prereqs.

-- Two more open deals at Negotiation (position 5, next stage terminal) so
-- steps 4/5/6 (Won / Lost-blank / Lost-with-reason) each get an
-- independent fixture instead of reusing "Stark Negotiation" three times.
INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, owner_id, organization_id, amount_minor, currency, source, captured_by)
VALUES
  ('00000000-0000-0000-0042-000000000007', '00000000-0000-0000-0000-000000000001',
   'Wayne Enterprises Renewal', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000005', '00000000-0000-0000-0010-000000000002',
   '00000000-0000-0000-0044-000000000001', 400000, 'EUR', 'seed', 'human:dev'),
  ('00000000-0000-0000-0042-000000000008', '00000000-0000-0000-0000-000000000001',
   'Wonka Industries Deal', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000005', '00000000-0000-0000-0010-000000000002',
   NULL, 600000, 'EUR', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- A closed (lost) deal fixture for the Reopen guide step. amount_minor is
-- NULL so deal_closed_fx (status<>open needs fx_rate_to_base when amount
-- is set) is trivially satisfied without a real FX freeze.
INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, owner_id, organization_id,
                   amount_minor, currency, source, captured_by, status, lost_reason, closed_at)
VALUES
  ('00000000-0000-0000-0042-000000000006', '00000000-0000-0000-0000-000000000001',
   'LexCorp Renewal', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000007', '00000000-0000-0000-0010-000000000002',
   '00000000-0000-0000-0044-000000000001', NULL, 'EUR', 'seed', 'human:dev',
   'lost', 'Budget cut', now() - interval '5 days')
ON CONFLICT (id) DO NOTHING;

-- Second + third stakeholder on "Umbrella Proposal" (already exists, Proposal
-- stage, zero stakeholders pre-T22) so it becomes the multi-threaded,
-- economic-buyer-present fixture for step 8. Champion=Carol, Economic
-- buyer=Bob.
INSERT INTO relationship (id, workspace_id, kind, person_id, deal_id, role, source, captured_by)
VALUES
  ('00000000-0000-0000-0043-000000000002', '00000000-0000-0000-0000-000000000001',
   'deal_stakeholder', '00000000-0000-0000-0001-000000000003',
   '00000000-0000-0000-0042-000000000004', 'champion', 'seed', 'human:dev'),
  ('00000000-0000-0000-0043-000000000003', '00000000-0000-0000-0000-000000000001',
   'deal_stakeholder', '00000000-0000-0000-0001-000000000002',
   '00000000-0000-0000-0042-000000000004', 'economic_buyer', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- Logged email/call/meeting activities + one open task (assignee = rep
-- app_user) on "Umbrella Proposal" for steps 10/12/13.
INSERT INTO activity (id, workspace_id, kind, subject, occurred_at, source_system, duration_seconds,
                       meeting_status, due_at, assignee_id, is_done, source, captured_by)
VALUES
  ('00000000-0000-0000-0045-000000000001', '00000000-0000-0000-0000-000000000001',
   'email', 'Proposal follow-up', now() - interval '3 days', 'gmail', NULL,
   NULL, NULL, NULL, false, 'seed', 'human:dev'),
  ('00000000-0000-0000-0045-000000000002', '00000000-0000-0000-0000-000000000001',
   'call', 'Pricing discussion', now() - interval '2 days', 'aircall', 900,
   NULL, NULL, NULL, false, 'seed', 'human:dev'),
  ('00000000-0000-0000-0045-000000000003', '00000000-0000-0000-0000-000000000001',
   'meeting', 'Proposal walkthrough', now() - interval '1 days', 'google_calendar', NULL,
   'held', NULL, NULL, false, 'seed', 'human:dev'),
  ('00000000-0000-0000-0045-000000000004', '00000000-0000-0000-0000-000000000001',
   'task', 'Send updated pricing sheet', now() - interval '1 days', NULL, NULL,
   NULL, now() + interval '3 days', '00000000-0000-0000-0010-000000000002', false, 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

INSERT INTO activity_link (id, workspace_id, activity_id, entity_type, deal_id)
VALUES
  ('00000000-0000-0000-0046-000000000001', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0045-000000000001', 'deal', '00000000-0000-0000-0042-000000000004'),
  ('00000000-0000-0000-0046-000000000002', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0045-000000000002', 'deal', '00000000-0000-0000-0042-000000000004'),
  ('00000000-0000-0000-0046-000000000003', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0045-000000000003', 'deal', '00000000-0000-0000-0042-000000000004'),
  ('00000000-0000-0000-0046-000000000004', '00000000-0000-0000-0000-000000000001',
   '00000000-0000-0000-0045-000000000004', 'deal', '00000000-0000-0000-0042-000000000004')
ON CONFLICT (id) DO NOTHING;

-- Stage-history fixture: "Umbrella Proposal" already sits at Proposal
-- (position 4) so it's genuinely mid-pipeline (New->Qualified->Discovery
-- already behind it) for step 1's stepper + step 11's history card.
INSERT INTO audit_log (workspace_id, actor_type, actor_id, action, entity_type, entity_id, before, after, occurred_at)
VALUES
  ('00000000-0000-0000-0000-000000000001'::uuid, 'human', '00000000-0000-0000-0010-000000000002',
   'create', 'deal', '00000000-0000-0000-0042-000000000004'::uuid,
   NULL, '{"name":"Umbrella Proposal","status":"open"}'::jsonb, now() - interval '10 days'),
  ('00000000-0000-0000-0000-000000000001'::uuid, 'human', '00000000-0000-0000-0010-000000000002',
   'advance_stage', 'deal', '00000000-0000-0000-0042-000000000004'::uuid,
   '{"stage_id":"00000000-0000-0000-0041-000000000001","status":"open"}'::jsonb,
   '{"stage_id":"00000000-0000-0000-0041-000000000002","status":"open"}'::jsonb, now() - interval '8 days'),
  ('00000000-0000-0000-0000-000000000001'::uuid, 'human', '00000000-0000-0000-0010-000000000002',
   'advance_stage', 'deal', '00000000-0000-0000-0042-000000000004'::uuid,
   '{"stage_id":"00000000-0000-0000-0041-000000000002","status":"open"}'::jsonb,
   '{"stage_id":"00000000-0000-0000-0041-000000000003","status":"open"}'::jsonb, now() - interval '6 days'),
  ('00000000-0000-0000-0000-000000000001'::uuid, 'human', '00000000-0000-0000-0010-000000000002',
   'advance_stage', 'deal', '00000000-0000-0000-0042-000000000004'::uuid,
   '{"stage_id":"00000000-0000-0000-0041-000000000003","status":"open"}'::jsonb,
   '{"stage_id":"00000000-0000-0000-0041-000000000004","status":"open"}'::jsonb, now() - interval '4 days');


COMMIT;
