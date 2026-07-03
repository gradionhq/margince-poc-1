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
  '{"person":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"organization":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"pipeline":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"stage":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"activity":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"lead":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"product":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"invoice":{"read":{"row_scope":"all"},"create":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}},"passport":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"approval":{"read":{"row_scope":"all"},"decide":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"},"curate":{"row_scope":"all"}},"conversation_link":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal_room":{"publish":{"row_scope":"all"}},"workspace":{"manage_members":{"row_scope":"all"},"export":{"row_scope":"all"},"import":{"row_scope":"all"}},"automation":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}}}'
),
(
  '00000000-0000-0000-0020-000000000002',
  '00000000-0000-0000-0000-000000000001',
  'rep', false,
  '{"person":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"organization":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"deal":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}},"stage":{"read":{"row_scope":"all"}},"activity":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"lead":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"product":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"invoice":{"read":{"row_scope":"own"},"create":{"row_scope":"own"}},"approval":{"read":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"}},"conversation_link":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"archive":{"row_scope":"own"}},"automation":{"read":{"row_scope":"own"},"create":{"row_scope":"own"},"update":{"row_scope":"own"},"archive":{"row_scope":"own"}}}'
),
(
  '00000000-0000-0000-0020-000000000003',
  '00000000-0000-0000-0000-000000000001',
  'read_only', false,
  '{"person":{"read":{"row_scope":"all"}},"organization":{"read":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"}},"stage":{"read":{"row_scope":"all"}},"activity":{"read":{"row_scope":"all"}},"lead":{"read":{"row_scope":"all"}},"product":{"read":{"row_scope":"all"}},"invoice":{"read":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}},"approval":{"read":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"all"}},"automation":{"read":{"row_scope":"all"}}}'
),
(
  '00000000-0000-0000-0020-000000000004',
  '00000000-0000-0000-0000-000000000001',
  'manager', false,
  '{"person":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"organization":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"stage":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}},"activity":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"lead":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"product":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"}},"invoice":{"read":{"row_scope":"all"},"create":{"row_scope":"all"}},"report":{"read":{"row_scope":"all"}},"approval":{"read":{"row_scope":"all"},"decide":{"row_scope":"all"}},"drafting_asset":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"},"curate":{"row_scope":"all"}},"conversation_link":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"archive":{"row_scope":"all"}},"deal_room":{"publish":{"row_scope":"all"}},"workspace":{"export":{"row_scope":"all"},"import":{"row_scope":"all"}},"automation":{"read":{"row_scope":"all"},"create":{"row_scope":"all"},"update":{"row_scope":"all"},"archive":{"row_scope":"all"}}}'
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

-- ─── Sample deals (one owned by rep, one by admin) ──────────
-- Gives the EP03 row-scope guides "own" vs "all" data to work with.

INSERT INTO deal (id, workspace_id, name, pipeline_id, stage_id, owner_id, source, captured_by)
VALUES
  ('00000000-0000-0000-0042-000000000001', '00000000-0000-0000-0000-000000000001',
   'Acme Expansion', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000001', '00000000-0000-0000-0010-000000000002', 'seed', 'human:dev'),
  ('00000000-0000-0000-0042-000000000002', '00000000-0000-0000-0000-000000000001',
   'Globex Renewal', '00000000-0000-0000-0040-000000000001',
   '00000000-0000-0000-0041-000000000002', '00000000-0000-0000-0010-000000000001', 'seed', 'human:dev')
ON CONFLICT (id) DO NOTHING;

-- ─── Deal stakeholder relationship (B-E09.9 UAT seed) ───────
-- Gives the stakeholders-per-deal cross-object report + its "Explain This Number"
-- drill-through a non-empty result to resolve (swarm/manual-test/b-e09-9.md Step 4).

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

COMMIT;
