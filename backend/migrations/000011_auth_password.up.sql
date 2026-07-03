-- 000011 — Add password_hash to app_user for local auth (EP03)
ALTER TABLE app_user ADD COLUMN password_hash text NULL;
