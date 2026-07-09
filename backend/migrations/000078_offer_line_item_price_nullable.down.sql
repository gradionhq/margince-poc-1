BEGIN;
-- Re-tightening assumes no live NULL rows exist (true for a fresh/test env). A production
-- rollback with live NULL rows would need an explicit backfill first — out of scope here.
ALTER TABLE offer_line_item ALTER COLUMN unit_price_minor SET NOT NULL;
COMMIT;
