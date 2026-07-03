-- 000054 — standalone single-column index on connector_secret.connection_id,
-- additive to the existing composite (connection_id, rotated_at DESC) index
-- from 000053. The composite index serves the Lookup query path (ORDER BY
-- rotated_at DESC); this one exists purely to satisfy the FK-index coverage
-- gate (referential_integrity_test.go), which requires a standalone index on
-- every FK column.
CREATE INDEX idx_connector_secret_connection_id_fk ON connector_secret (connection_id);
