ALTER TABLE workspace DROP CONSTRAINT IF EXISTS workspace_sor_mode_incumbent_chk;
ALTER TABLE workspace DROP COLUMN IF EXISTS incumbent;
ALTER TABLE workspace DROP COLUMN IF EXISTS sor_mode;
