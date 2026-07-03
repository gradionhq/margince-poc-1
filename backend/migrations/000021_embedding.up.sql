BEGIN;
CREATE TABLE embedding (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id  uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  source_type   text NOT NULL,          -- 'person' | 'organization' | 'deal' | 'activity' | 'lead'
  source_id     uuid NOT NULL,          -- the core record this embeds
  content_hash  text NOT NULL,          -- sha256 of source text; never recompute unless this changes
  embedding     vector(1024) NOT NULL,  -- at-rest-encrypted at the storage layer; sized to EmbedDims
  dims          int  NOT NULL,          -- actual dims of the stored vector (detect model mismatch)
  source        text NOT NULL,          -- provenance (P5)
  captured_by   text NOT NULL,          -- provenance (P5)
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, source_type, source_id)
);
CREATE INDEX idx_embedding_hnsw ON embedding USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_embedding_source ON embedding (workspace_id, source_type, source_id);
-- Standalone single-column FK index (satisfies the FKColumnsAreIndexed invariant);
-- the composite above does not match the single-column (workspace_id) requirement.
CREATE INDEX idx_embedding_ws ON embedding (workspace_id);

ALTER TABLE embedding ENABLE ROW LEVEL SECURITY;
ALTER TABLE embedding FORCE ROW LEVEL SECURITY;
CREATE POLICY embedding_tenant_isolation ON embedding
  USING      (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid)
  WITH CHECK (workspace_id = nullif(current_setting('app.workspace_id', true), '')::uuid);
-- NOTE: ALTER DEFAULT PRIVILEGES in 000004 already grants CRUD on new public tables
-- to margince_app, so this explicit GRANT is redundant-but-harmless (kept for clarity).
GRANT SELECT, INSERT, UPDATE, DELETE ON embedding TO margince_app;
COMMIT;
