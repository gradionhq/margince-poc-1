-- 000071 down — reverse of the up migration, FK-safe order: offer_line_item references
-- offer/product; offer references offer_template. Plain DROP TABLE also drops each table's
-- own indexes, policies, and triggers.
DROP TABLE IF EXISTS offer_line_item;
DROP TABLE IF EXISTS offer;
DROP TABLE IF EXISTS offer_template;
DROP TABLE IF EXISTS product;
