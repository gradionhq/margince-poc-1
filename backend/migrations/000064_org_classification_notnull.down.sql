ALTER TABLE organization
  ALTER COLUMN classification DROP NOT NULL,
  ALTER COLUMN classification DROP DEFAULT;
