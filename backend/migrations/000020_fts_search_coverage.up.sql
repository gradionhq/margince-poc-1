BEGIN;
-- tsvector columns + GIN indexes already exist on person/organization/deal/lead/activity
-- (000003, 000008). This migration ensures the engine's query path has a consistent,
-- documented surface: a unified search view spanning the core objects + activities,
-- each row carrying (workspace_id, entity_type, entity_id, search_tsv, snippet_source).
-- The view inherits RLS from its base tables (security_invoker), so a cross-tenant
-- query returns 0 foreign rows automatically.
CREATE OR REPLACE VIEW search_corpus
  WITH (security_invoker = true) AS
    SELECT workspace_id, 'person'::text       AS entity_type, id AS entity_id,
           search_tsv, coalesce(full_name,'') AS snippet FROM person       WHERE archived_at IS NULL
  UNION ALL
    SELECT workspace_id, 'organization'::text, id, search_tsv, coalesce(name,'')   FROM organization WHERE archived_at IS NULL
  UNION ALL
    SELECT workspace_id, 'deal'::text,         id, search_tsv, coalesce(name,'')   FROM deal         WHERE archived_at IS NULL
  UNION ALL
    SELECT workspace_id, 'activity'::text,     id, search_tsv, coalesce(subject,'') FROM activity    WHERE archived_at IS NULL
  UNION ALL
    SELECT workspace_id, 'lead'::text,         id, search_tsv, coalesce(full_name, company_name, '') FROM lead WHERE archived_at IS NULL;
-- column-name reality (verified against 000003/000008):
--   person.full_name, organization.name, deal.name, activity.subject, lead.full_name|company_name (NO lead.name).
-- The first UNION arm's column names win, so the snippet column is named `snippet` view-wide.
COMMIT;
