-- 000010 — Add missing single-column FK indexes for referential-integrity coverage.
-- Postgres does not auto-create indexes on FK child columns; each FK column needs
-- its own index so ON DELETE CASCADE / ON DELETE RESTRICT scans are efficient.

-- Pre-existing tables (000003_core_objects) ---------------------------------

CREATE INDEX IF NOT EXISTS idx_lead_owner        ON lead (owner_id)         WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_person_owner_fk  ON person (owner_id)       WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_person_email_ws  ON person_email (workspace_id);
CREATE INDEX IF NOT EXISTS idx_org_owner_single ON organization (owner_id)  WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_stage_ws         ON stage (workspace_id);
CREATE INDEX IF NOT EXISTS idx_deal_pipeline_fk ON deal (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_deal_owner_fk    ON deal (owner_id)          WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_deal_partner_org ON deal (partner_org_id)  WHERE partner_org_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_activity_ws      ON activity (workspace_id);
CREATE INDEX IF NOT EXISTS idx_activity_assignee ON activity (assignee_id) WHERE assignee_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_activity_link_ws ON activity_link (workspace_id);
CREATE INDEX IF NOT EXISTS idx_activity_link_act ON activity_link (activity_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_ws     ON audit_log (workspace_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_behalf ON audit_log (on_behalf_of) WHERE on_behalf_of IS NOT NULL;

-- 000005_rbac ----------------------------------------------------------------

CREATE INDEX IF NOT EXISTS idx_team_parent          ON team (parent_team_id)       WHERE parent_team_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_team_membership_ws   ON team_membership (workspace_id);
CREATE INDEX IF NOT EXISTS idx_team_membership_uid  ON team_membership (user_id);
CREATE INDEX IF NOT EXISTS idx_role_assignment_ws   ON role_assignment (workspace_id);
CREATE INDEX IF NOT EXISTS idx_role_assignment_role ON role_assignment (role_id);
CREATE INDEX IF NOT EXISTS idx_role_assignment_uid  ON role_assignment (user_id);
CREATE INDEX IF NOT EXISTS idx_role_assignment_team ON role_assignment (team_id)   WHERE team_id IS NOT NULL;

-- 000006_org_gaps_person_phone -----------------------------------------------

CREATE INDEX IF NOT EXISTS idx_org_domain_ws   ON organization_domain (workspace_id);
CREATE INDEX IF NOT EXISTS idx_person_phone_ws ON person_phone (workspace_id);

-- 000007_partner_relationship ------------------------------------------------

CREATE INDEX IF NOT EXISTS idx_relationship_ws              ON relationship (workspace_id);
CREATE INDEX IF NOT EXISTS idx_relationship_person          ON relationship (person_id)           WHERE person_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_relationship_org             ON relationship (organization_id)     WHERE organization_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_relationship_counterparty    ON relationship (counterparty_org_id) WHERE counterparty_org_id IS NOT NULL;

-- 000008_deal_history_fx_lead ------------------------------------------------

CREATE INDEX IF NOT EXISTS idx_deal_stage_history_ws2       ON deal_stage_history (workspace_id);
CREATE INDEX IF NOT EXISTS idx_deal_stage_history_deal2     ON deal_stage_history (deal_id);
CREATE INDEX IF NOT EXISTS idx_deal_stage_history_from_stg  ON deal_stage_history (from_stage_id)  WHERE from_stage_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_deal_stage_history_to_stg    ON deal_stage_history (to_stage_id);
CREATE INDEX IF NOT EXISTS idx_fx_rate_ws                   ON fx_rate (workspace_id);
CREATE INDEX IF NOT EXISTS idx_lead_promoted_person         ON lead (promoted_person_id)           WHERE promoted_person_id IS NOT NULL;

-- 000009_lists_tags_attachments ----------------------------------------------

CREATE INDEX IF NOT EXISTS idx_list_owner      ON list (owner_id)    WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_list_team       ON list (team_id)     WHERE team_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_list_member_ws  ON list_member (workspace_id);
CREATE INDEX IF NOT EXISTS idx_taggable_ws     ON taggable (workspace_id);
CREATE INDEX IF NOT EXISTS idx_attachment_ws   ON attachment (workspace_id);
