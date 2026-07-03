ALTER TABLE workspace
  ADD COLUMN sor_mode  text NOT NULL DEFAULT 'native'
    CHECK (sor_mode IN ('native','overlay')),
  ADD COLUMN incumbent text;

ALTER TABLE workspace
  ADD CONSTRAINT workspace_sor_mode_incumbent_chk
    CHECK ((sor_mode = 'overlay') = (incumbent IS NOT NULL));
