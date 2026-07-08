-- 000073 down — reverse of the up migration. No inter-table FK ordering constraint among the
-- three tables (none references another); plain DROP TABLE also drops each table's own
-- indexes, policies, and triggers.
DROP TABLE IF EXISTS bulk_operation;
DROP TABLE IF EXISTS quota;
DROP TABLE IF EXISTS attachment;
